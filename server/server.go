package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"runtime"
	"strconv"
	"sync"
	"time"

	"code.google.com/p/draw2d/draw2d"
	"golang.org/x/net/websocket"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	port      string
	genetrees GeneTrees
)

// Post Data structs
type Loc struct {
	From float64 `json:"from"`
	To   float64 `json:"to"`
}

type Track struct {
	Name    string  `json:"name"`
	Height  float64 `json:"height"`
	VOffset float64 `json:"v_offset"`
	FgColor *Color  `json:"fgColor"`
	BgColor *Color  `json:"bgColor"`
}

type Tracks []*Track // []*Track? performance?

type Color struct {
	R uint8 `json:"r"`
	G uint8 `json:"g"`
	B uint8 `json:"b"`
}

func (c *Color) RGBA() (r, g, b, a uint32) {
	return color.NRGBA{R: c.R, G: c.G, B: c.B, A: 0xFF}.RGBA()
}

type Conf struct {
	Height  float64 `json:"height"`
	Width   float64 `json:"width"`
	BgColor *Color  `json:"bgColor"`
}

type PostData struct {
	Loc    *Loc `json:"loc"` // *Loc? Use pointer?
	Tracks Tracks
	Conf   *Conf `json:"conf"` // *Conf? Use pointer?
}

type Gap struct {
	Start float64 `bson:"start" json:"start"`
	End   float64 `bson:"end" json:"end"`
	Type  string  `bson:"type" json:"type"`
}

type Gaps []*Gap

type DbDatum struct {
	Id             string    `bson:"_id"`
	Seq            string    `bson:"subseq"`
	ExonBoundaries []float64 `bson:"exon_boundaries"`
	Gaps           Gaps      `bson:"gaps"`
	// Length         uint32  `bson:"length"`
	// nExonBounds    uint32  `bson:"nexon_bounds"`
	// nGaps          uint32  `bson:"ngaps"`
}

type ImgData struct {
	DbData *DbDatum
	Track  *Track
	Conf   *Conf
	Loc    *Loc
}

func init() {
	flag.StringVar(&port, "port", "1338", "Port to listen (defaults to 1338)")
	flag.Parse()
	runtime.GOMAXPROCS(runtime.NumCPU())
}

func getPostData(r *http.Request) *PostData {
	// Decode the json body
	decoder := json.NewDecoder(r.Body)
	postData := &PostData{}
	err := decoder.Decode(postData)
	checkError("Can't parse post data: ", err)
	return postData
}

func limit(w http.ResponseWriter, r *http.Request, collection *mgo.Collection) {
	// mongo query:
	// WARNING!! This assumes only one tree in the db. We need to first $match the correct genetree stable id
	// we need a starting $match in the pipeline
	// db.annot.aggregate({
	// 	$group:{
	// 		"_id":"$genetree",
	// 		"maxL": {
	// 			"$max":"$length"
	// 		}
	// 	}
	// })
	resLim := struct {
		MaxL int `bson:"maxL" json:"limit"`
	}{}
	group := bson.M{"$group": bson.M{
		"_id":  "$genetree",
		"maxL": bson.M{"$max": "$length"}}}
	pipeline := []bson.M{
		group}
	iter := collection.Pipe(pipeline).Iter()
	iter.Next(&resLim)
	err := iter.Close()
	checkError("Problem closing iter: ", err)

	j, err := json.Marshal(resLim)
	checkError("Problem marshaling json: ", err)
	n, err := w.Write(j)
	checkError("Problem sending limits back to client: ", err)
	debug("%d bytes transferred successfuly (from limit)", n)
}

func getDbData(l *Loc, c *Conf, t *Track, collection *mgo.Collection, res chan<- ImgData) { // TODO: Do we need to pass conf?
	// mongo query
	// db.annot.aggregate(
	//      {$match  : {"id" : "ENSDORG00000004235"}},
	// 	{$unwind : "$gaps"},
	// 	{$unwind : "$exon_boundaries"},
	// 	{$match: {
	// 		$or :[
	// 			{"gaps.start" : {$gte : from, $lte:to}},
	// 			{"gaps.end" : {$gte : from, $lte:to}},
	//                      {"exon_boundaries" : {$gte : from, $lte:to}}
	// 	             ]
	// 	         }
	// 	},
	// 	{$project:{
	// 		id:1,
	// 		gaps:1,
	// 		exon_boundaries:1,
	// 		subseq:{$substr:["$seq", from, (to-from)]}}},
	// 	{$group : {
	// 		_id : "$id",
	// 		gaps : {"$addToSet" : "$gaps"},
	// 		exon_boundaries : {"$addToSet" : "$exon_boundaries"},
	// 		subseq : {"$first" : "$subseq"}}},
	// 	{$project : {
	// 		id:1,
	// 		gaps:1,
	// 		exon_boundaries:1,
	// 		subseq:1,
	// 		ngaps:{"$size":"$gaps"},
	// 		nexon_bounds:{"$size" : "$exon_boundaries"}}}
	// )
	dbT1 := time.Now()
	matchID := bson.M{"$match": bson.M{"id": t.Name}}
	unwGaps := bson.M{"$unwind": "$gaps"}
	unwEB := bson.M{"$unwind": "$exon_boundaries"}
	matchRange := bson.M{"$gte": l.From, "$lte": l.To}
	gapsStart := bson.M{"gaps.start": matchRange}
	gapsEnd := bson.M{"gaps.end": matchRange}
	eb := bson.M{"exon_boundaries": matchRange}
	or := []bson.M{gapsStart, gapsEnd, eb}
	matchGaps := bson.M{"$match": bson.M{"$or": or}}
	project := bson.M{"$project": bson.M{
		"id":              1,
		"gaps":            1,
		"exon_boundaries": 1,
		"subseq": bson.M{"$substr": []interface{}{
			"$seq",
			l.From,
			(l.To - l.From)}}}}
	group := bson.M{"$group": bson.M{
		"_id":             "$id",
		"gaps":            bson.M{"$addToSet": "$gaps"},
		"exon_boundaries": bson.M{"$addToSet": "$exon_boundaries"},
		"subseq":          bson.M{"$first": "$subseq"}}}

	pipeline := []bson.M{
		matchID,
		unwGaps,
		unwEB,
		matchGaps,
		project,
		group}

	pipe := collection.Pipe(pipeline)
	iter := pipe.Iter()
	record := &DbDatum{}
	iter.Next(record)

	if err := iter.Close(); err != nil {
		checkError("Closing iterator... ", err)
	}

	res <- ImgData{DbData: record, Track: t, Conf: c, Loc: l}
	dbT2 := time.Now()
	debug(fmt.Sprintf("Time to retrieve %s track data from db: %v\n", t.Name, dbT2.Sub(dbT1)))
	return
}

// scale returns a function that transforms domain coordinates in range coordinates.
func scale(rng []float64, dmn []float64) func(float64) float64 {
	rangeFrom := rng[0]
	rangeTo := rng[1]
	domainFrom := dmn[0]
	domainTo := dmn[1]

	b := (rangeTo - rangeFrom) / (domainTo - domainFrom)
	// x' = (width / (from-to)) * (x-from)
	f := func(pos float64) float64 {
		// fmt.Printf("%d\n", float64(b*(float64(pos-domainFrom))))
		return float64(b * (pos - domainFrom))
	}

	return f
}

func drawImg(data ImgData, img *image.RGBA, scaleFn func(float64) float64, wg *sync.WaitGroup) {
	drawT1 := time.Now()
	gc := draw2d.NewGraphicContext(img)
	fgColor := data.Track.FgColor
	highColor := &Color{R: 0, G: 100, B: 0}
	lowColor := &Color{R: 154, G: 205, B: 50}
	boundColor := &Color{R: 205, G: 0, B: 0}
	vOffset := data.Track.VOffset
	trackOffset := data.Track.Height / 8
	upLim := vOffset + trackOffset
	downLim := vOffset + (data.Track.Height - (2 * trackOffset))

	// Draw the horizontal guides
	gc.MoveTo(0, upLim)
	gc.LineTo(data.Conf.Width, upLim)
	gc.MoveTo(0, downLim)
	gc.LineTo(data.Conf.Width, downLim)
	gc.SetStrokeColor(fgColor)
	gc.Stroke()

	// Draw the gaps blocks
	for _, block := range data.DbData.Gaps {
		block_start := scaleFn(block.Start)
		block_end := scaleFn(block.End)
		draw2d.Rect(gc, block_start, (vOffset + trackOffset), block_end, vOffset+(data.Track.Height-(2*trackOffset)))

		if block.Type == "low" {
			gc.SetFillColor(lowColor)
		} else if block.Type == "high" {
			gc.SetFillColor(highColor)
		} else {
			panic("Unknown gap type: " + block.Type)
		}
		gc.Fill()
		gc.Stroke()
	}

	// Draw the exon boundaries
	for _, bound := range data.DbData.ExonBoundaries {
		where := scaleFn(bound)
		gc.MoveTo(where, vOffset)
		gc.LineTo(where, vOffset+data.Track.Height)
		gc.SetStrokeColor(boundColor)
		gc.Stroke()
	}

	// Draw sequences if zoomed enough
	if (data.Loc.To - data.Loc.To) < 300 {

	}
	defer wg.Done()
	drawT2 := time.Now()
	debug(fmt.Sprintf("Time to draw 1 track: %v\n", drawT2.Sub(drawT1)))
	return
}

func httpImage(w http.ResponseWriter, r *http.Request, collection *mgo.Collection) {
	var postData = getPostData(r)

	// scaleFn transform domain coordinates into range coordinates
	scaleFn := scale([]float64{0, postData.Conf.Width}, []float64{postData.Loc.From, postData.Loc.To})

	// New Image
	img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(postData.Conf.Width), int(postData.Conf.Height))})

	// dataChan collects all the db records for a given track
	dataChan := make(chan ImgData, len(postData.Tracks))
	dbt1 := time.Now()
	for _, track := range postData.Tracks {
		go getDbData(postData.Loc, postData.Conf, track, collection, dataChan) // Sends to dataChan
	}

	var wg sync.WaitGroup
	wg.Add(len(postData.Tracks))
	for i := 0; i < len(postData.Tracks); i++ {
		data := <-dataChan
		go drawImg(data, img, scaleFn, &wg)
	}

	fmt.Print("Waiting...")
	wg.Wait()
	fmt.Println("... Done")
	dbt2 := time.Now()
	fmt.Printf("Time to read all the records from db: %v", dbt2.Sub(dbt1))

	//saveToPngFile("image.png", img)
	// Send the image back to the server

	// Send to the client
	var png_buffer bytes.Buffer
	err := png.Encode(&png_buffer, img)
	checkError("Can't encode img in png format: ", err)
	data_uri := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png_buffer.Bytes()))

	w.Header().Set("Content-Length", strconv.Itoa(len(data_uri)))
	// TODO: Which one is faster?
	n, err := w.Write([]byte(data_uri))
	// n, err := io.Copy(w, bytes.NewReader([]byte(data_uri)))
	checkError("Problem sending data to client: ", err)
	debug("%d bytes transferred successfuly", n)
}

func wsImage(ws *websocket.Conn, collection *mgo.Collection) {
	for {
		wsData := &PostData{}
		err := websocket.JSON.Receive(ws, wsData)
		checkError("Problem reading data from websocket: ", err)

		// scaleFn transform domain coordinates into range coordinates
		scaleFn := scale([]float64{0, wsData.Conf.Width}, []float64{wsData.Loc.From, wsData.Loc.To})

		// New Image
		img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(wsData.Conf.Width), int(wsData.Conf.Height))})

		dataChan := make(chan ImgData, len(wsData.Tracks))
		dbt1 := time.Now()
		for _, track := range wsData.Tracks {
			go getDbData(wsData.Loc, wsData.Conf, track, collection, dataChan) // Sends to dataChan
		}

		var wg sync.WaitGroup
		wg.Add(len(wsData.Tracks))
		for i := 0; i < len(wsData.Tracks); i++ {
			data := <-dataChan
			go drawImg(data, img, scaleFn, &wg)
		}

		fmt.Print("Waiting...")
		wg.Wait()
		fmt.Println("... Done")
		dbt2 := time.Now()
		debug(fmt.Sprintf("Time to create everything: %v", dbt2.Sub(dbt1)))

		// Send to the client
		var png_buffer bytes.Buffer
		err = png.Encode(&png_buffer, img)
		checkError("Can't encode img in png format: ", err)
		data_uri := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png_buffer.Bytes()))
		err = websocket.Message.Send(ws, data_uri)
		checkError("Problem sending data to client: ", err)
	}
}

func main() {
	// MongoDB Connection
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	// TODO: Do these affect performance?
	// db.SetBatch(1000)
	// db.SetPrefetch(0.75)
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	// Handle static content
	http.Handle("/", http.FileServer(http.Dir("../theme")))

	// Handle limits on space
	http.HandleFunc("/limit", func(w http.ResponseWriter, r *http.Request) {
		limit(w, r, collection)
	})

	// Handle websocket communication
	http.Handle("/boardws", websocket.Handler(func(ws *websocket.Conn) {
		wsImage(ws, collection)
	}))

	// Handle http communication
	http.HandleFunc("/board", func(w http.ResponseWriter, r *http.Request) {
		httpImage(w, r, collection)
	})
	debug("Listening to port: %s", port)
	http.ListenAndServe(":1338", nil)
}
