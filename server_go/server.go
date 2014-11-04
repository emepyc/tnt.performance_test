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
	"io"
	"net/http"
	"sync"

	"code.google.com/p/draw2d/draw2d"
	"code.google.com/p/go.net/websocket"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	port string
	// collection *mgo.Collection
	// img        *draw2d.ImageGraphicContext
)

type Coord uint32

// Post Data structs
type Loc struct {
	From Coord `json:"from"`
	To   Coord `json:"to"`
}

type Track struct {
	Name    string `json:"name"`
	Height  Coord  `json:"height"`
	VOffset Coord  `json:"v_offset"`
	FgColor *Color `json:"fgColor"`
	BgColor *Color `json:"bgColor"`
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
	Height  Coord  `json:"height"`
	Width   Coord  `json:"width"`
	BgColor *Color `json:"bgColor"`
}

type PostData struct {
	Loc    *Loc `json:"loc"` // *Loc? Use pointer?
	Tracks Tracks
	Conf   *Conf `json:"conf"` // *Conf? Use pointer?
}

// DB Data structs
type DbDatum struct {
	Name  string
	Start Coord
	End   Coord
}

type ImgData struct {
	DbData []*DbDatum
	Track  *Track
	Conf   *Conf
}

func init() {
	flag.StringVar(&port, "port", "1338", "Port to listen (defaults to 1338)")
	flag.Parse()
}

func getPostData(r *http.Request) *PostData {
	// Decode the json body
	decoder := json.NewDecoder(r.Body)
	postData := &PostData{}
	err := decoder.Decode(postData)
	checkError("Can't parse post data: ", err)
	return postData
}

// func plotData(dat chan *DbData)

func getDbData(l *Loc, c *Conf, t *Track, collection *mgo.Collection, res chan<- ImgData) { // TODO: Do we need to pass conf?
	// We want: db.testData.find({name : "track_1", $or : [{start : {$gte : 22, $lte:50}}, {end : {$gte : 22, $lte : 50}}]
	s := bson.M{"start": bson.M{"$gte": l.From, "$lte": l.To}}
	e := bson.M{"end": bson.M{"$gte": l.From, "$lte": l.To}}
	r := []bson.M{s, e}

	query := collection.Find(bson.M{"name": t.Name, "$or": r})

	count, err := query.Count()
	checkError("Problem retrieving count of rows: ", err)
	row_iter := query.Iter()

	records := make([]*DbDatum, count)
	err = row_iter.All(&records)
	checkError("Problem retrieving all the records: ", err)
	// fmt.Println(records)

	res <- ImgData{DbData: records, Track: t, Conf: c}
	return
}

// scale returns a function that transforms domain coordinates in range coordinates.
func scale(rng []Coord, dmn []Coord) func(Coord) Coord {
	rangeFrom := rng[0]
	rangeTo := rng[1]
	domainFrom := dmn[0]
	domainTo := dmn[1]

	b := ((float64(rangeTo) - float64(rangeFrom)) / (float64(domainTo) - float64(domainFrom)))
	// x' = (width / (from-to)) * (x-from)
	f := func(pos Coord) Coord {
		// fmt.Printf("%d\n", Coord(b*(float64(pos-domainFrom))))
		return Coord(b * float64((pos - domainFrom)))
	}

	return f
}

func drawImg(data ImgData, img *image.RGBA, scaleFn func(Coord) Coord, wg *sync.WaitGroup) {
	gc := draw2d.NewGraphicContext(img)
	fgColor := data.Track.FgColor
	for _, block := range data.DbData {
		block_start := scaleFn(block.Start)
		block_end := scaleFn(block.End)
		vOffset := int(data.Track.VOffset)
		// TODO: Do all these conversions before hand?
		// gc.MoveTo(float64(block_start), float64(vOffset))
		draw2d.Rect(gc, float64(block_start), float64(vOffset), float64(block_end), float64(vOffset+int(data.Track.Height)))
		// gc.LineTo(float64(block_end), float64(vOffset))
		// gc.LineTo(float64(block_end), float64(vOffset+int(data.Track.Height)))
		// gc.LineTo(float64(block_start), float64(vOffset+int(data.Track.Height)))
		// gc.Close()
		gc.SetFillColor(fgColor)
		gc.Fill()
		gc.Stroke()

	}
	defer wg.Done()
	return
}

type netReceiver interface {
	Receive() *PostData
}

type httpReceiver struct {
	R *http.Request
}

func (r httpReceiver) Receive() *PostData {
	debug("Reading from http... OK")
	defer r.R.Body.Close()
	decoder := json.NewDecoder(r.R.Body)
	postData := &PostData{}
	err := decoder.Decode(postData)
	checkError("Can't parse post data: ", err)
	return postData
}

type wsReceiver struct {
	R *websocket.Conn
}

func (r wsReceiver) Receive() *PostData {
	debug("Reading from websocket... OK")
	wsData := &PostData{}
	err := websocket.JSON.Receive(r.R, wsData)
	checkError("Problem reading data from websocket: ", err)
	return wsData
}

//func getImage(w http.ResponseWriter, r *http.Request, collection *mgo.Collection) {
//func getImage(w http.ResponseWriter, r netReceiver, collection *mgo.Collection) {
func httpImage(w io.Writer, r netReceiver, collection *mgo.Collection) {
	// var postData = getPostData(r)
	postData := r.Receive()
	// scaleFn transform domain coordinates into range coordinates
	scaleFn := scale([]Coord{0, postData.Conf.Width}, []Coord{postData.Loc.From, postData.Loc.To})

	// New Image
	img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(postData.Conf.Width), int(postData.Conf.Height))})

	// dataChan collects all the db records for a given track
	dataChan := make(chan ImgData, len(postData.Tracks))
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

	//saveToPngFile("image.png", img)
	// Send the image back to the server

	// Send to the client
	var png_buffer bytes.Buffer
	err := png.Encode(&png_buffer, img)
	checkError("Can't encode img in png format: ", err)
	data_uri := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png_buffer.Bytes()))
	n, err := w.Write([]byte(data_uri))
	checkError("Problem sending data to client: ", err)
	debug("%d bytes transferred successfuly", n)
}

func wsImage(ws *websocket.Conn, collection *mgo.Collection) {
	for {
		wsData := &PostData{}
		err := websocket.JSON.Receive(ws, wsData)
		checkError("Problem reading data from websocket: ", err)

		// scaleFn transform domain coordinates into range coordinates
		scaleFn := scale([]Coord{0, wsData.Conf.Width}, []Coord{wsData.Loc.From, wsData.Loc.To})

		// New Image
		img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(wsData.Conf.Width), int(wsData.Conf.Height))})

		dataChan := make(chan ImgData, len(wsData.Tracks))
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
	db, err := mgo.Dial("mongodb://127.0.0.1:27017")
	// TODO: Do these affect performance?
	// db.SetBatch(1000)
	// db.SetPrefetch(0.75)
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("mytestdb").C("testData")

	http.Handle("/", http.FileServer(http.Dir("../theme")))

	http.Handle("/boardws", websocket.Handler(func(ws *websocket.Conn) {
		wsImage(ws, collection)
	}))
	http.HandleFunc("/board", func(w http.ResponseWriter, r *http.Request) {
		httpImage(w, httpReceiver{R: r}, collection)
	})
	debug("Listening to port: %s", port)
	http.ListenAndServe(":1338", nil)
}
