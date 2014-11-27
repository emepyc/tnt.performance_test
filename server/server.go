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
	"log"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"strconv"
	"sync"
	"time"
)

// Node caching
type Nodes map[string]*NodeDatum // NodeDatum? GC performance?

var (
	port  string
	nodes Nodes
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

type NodeDatum struct {
	Id             string    `bson:"_id"`
	Seq            string    `bson:"subseq"`
	ExonBoundaries []float64 `bson:"exon_boundaries"`
	Gaps           Gaps      `bson:"gaps"`
	// Length         uint32  `bson:"length"`
	// nExonBounds    uint32  `bson:"nexon_bounds"`
	// nGaps          uint32  `bson:"ngaps"`
}

type ImgData struct {
	NodeData *NodeDatum
	Track    *Track
	Conf     *Conf
	Loc      *Loc
	Img      *image.RGBA
	Scale    func(float64) float64
}

func init() {
	// Flag parsing
	flag.StringVar(&port, "port", "1338", "Port to listen (defaults to 1338)")
	flag.Parse()

	// Set cpus
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Initialize GeneTrees (Cache)
	nodes = make(Nodes) // make(Nodes, 1000)
}

func getPostData(r *http.Request) *PostData {
	// Decode the json body
	decoder := json.NewDecoder(r.Body)
	postData := &PostData{}
	err := decoder.Decode(postData)
	checkError("Can't parse post data: ", err)
	return postData
}

func getNode(nodeId string) *NodeDatum {
	// 1. Cache lookup
	if node, ok := nodes[nodeId]; ok {
		return node
	}

	// 2. Fetch from db if not in cache
	node := fetchNode(nodeId)
	nodes[nodeId] = node
	return node
}

func filterNodeData(node *NodeDatum, loc *Loc) *NodeDatum {
	// Gaps
	gaps := node.Gaps
	var filtGaps Gaps
	for _, gap := range gaps {
		if (gap.Start > loc.From) && (gap.Start < loc.To) ||
			(gap.End > loc.From) && (gap.End < loc.From) ||
			(gap.Start < loc.From) && (gap.End > loc.To) {
			filtGaps = append(filtGaps, gap)
		}
	}

	// Exon Boundaries
	ebs := node.ExonBoundaries
	var filtEBs []float64
	for _, eb := range ebs {
		if (eb > loc.From) && (eb < loc.To) {
			filtEBs = append(filtEBs, eb)
		}
	}

	// Seq
	seq := node.Seq
	subseq := ""
	if len(seq) >= int(loc.To) {
		from := int(loc.From)
		to := int(loc.To)
		subseq = seq[from:(to - from)]
	}

	return &NodeDatum{
		Id:             node.Id,
		Seq:            subseq,
		ExonBoundaries: filtEBs,
		Gaps:           filtGaps}
}

func processTrack(trackImgData *ImgData, wg *sync.WaitGroup) {
	// Get the node data
	node := getNode(trackImgData.Track.Name)

	// Filter
	filtNode := filterNodeData(node, trackImgData.Loc)
	trackImgData.NodeData = filtNode

	// Draw
	drawTrack(trackImgData)

	wg.Done()
	return
}

func httpImage(w http.ResponseWriter, r *http.Request) {
	var postData = getPostData(r)

	// scaleFn transform domain coordinates into range coordinates
	scaleFn := scale([]float64{0, postData.Conf.Width}, []float64{postData.Loc.From, postData.Loc.To})

	// New Image
	img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(postData.Conf.Width), int(postData.Conf.Height))})

	debug("Firing track processing (%d goroutines)", len(postData.Tracks))

	t1 := time.Now()

	f, err := os.Create("last.prof")
	if err != nil {
		log.Fatal(err)
	}
	pprof.StartCPUProfile(f)
	defer pprof.StopCPUProfile()

	var wg sync.WaitGroup
	wg.Add(len(postData.Tracks))
	for _, track := range postData.Tracks {
		trackImgData := &ImgData{
			Track: track,
			Conf:  postData.Conf,
			Loc:   postData.Loc,
			Img:   img,
			Scale: scaleFn}
		go processTrack(trackImgData, &wg)
	}
	wg.Wait()

	t2 := time.Now()
	debug(fmt.Sprintf("Time to process all the tracks: %v\n", t2.Sub(t1)))

	debug("All process done")

	// Send img to the client
	var png_buffer bytes.Buffer
	err = png.Encode(&png_buffer, img)
	checkError("Can't encode img in png format: ", err)
	data_uri := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png_buffer.Bytes()))

	w.Header().Set("Content-Length", strconv.Itoa(len(data_uri)))
	n, err := w.Write([]byte(data_uri))
	// n, err := io.Copy(w, bytes.NewReader([]byte(data_uri)))
	// checkError("Problem sending data to client: ", err)
	debug("%d bytes transferred successfuly", n)

	// fmt.Println("SAVE kk.png")
	// saveToPngFile("./kk.png", img)

}

// func wsImage(ws *websocket.Conn, collection *mgo.Collection) {
// 	for {
// 		wsData := &PostData{}
// 		err := websocket.JSON.Receive(ws, wsData)
// 		checkError("Problem reading data from websocket: ", err)

// 		// scaleFn transform domain coordinates into range coordinates
// 		scaleFn := scale([]float64{0, wsData.Conf.Width}, []float64{wsData.Loc.From, wsData.Loc.To})

// 		// New Image
// 		img := image.NewRGBA(image.Rectangle{image.Pt(0, 0), image.Pt(int(wsData.Conf.Width), int(wsData.Conf.Height))})

// 		dataChan := make(chan ImgData, len(wsData.Tracks))
// 		dbt1 := time.Now()
// 		for _, track := range wsData.Tracks {
// 			go getDbData(wsData.Loc, wsData.Conf, track, collection, dataChan) // Sends to dataChan
// 		}

// 		var wg sync.WaitGroup
// 		wg.Add(len(wsData.Tracks))
// 		for i := 0; i < len(wsData.Tracks); i++ {
// 			data := <-dataChan
// 			go drawImg(data, img, scaleFn, &wg)
// 		}

// 		fmt.Print("Waiting...")
// 		wg.Wait()
// 		fmt.Println("... Done")
// 		dbt2 := time.Now()
// 		debug(fmt.Sprintf("Time to create everything: %v", dbt2.Sub(dbt1)))

// 		// Send to the client
// 		var png_buffer bytes.Buffer
// 		err = png.Encode(&png_buffer, img)
// 		checkError("Can't encode img in png format: ", err)
// 		data_uri := fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(png_buffer.Bytes()))
// 		err = websocket.Message.Send(ws, data_uri)
// 		checkError("Problem sending data to client: ", err)
// 	}
// }

func limit(w http.ResponseWriter, r *http.Request) {
	nodeId := r.FormValue("node")

	jsonLim := fetchNodeSeqLength(nodeId)

	n, err := w.Write(jsonLim)
	checkError("Problem sending limits back to client: ", err)
	debug("%d bytes transferred successfuly (from limit)", n)
}

func main() {
	// Handle static content
	http.Handle("/", http.FileServer(http.Dir("../theme")))

	// Handle limits on space
	http.HandleFunc("/limit", limit)

	// Handle http communication
	http.HandleFunc("/board", httpImage)

	// Handle websocket communication
	// http.Handle("/boardws", websocket.Handler(func(ws *websocket.Conn) {
	// 	wsImage(ws, collection)
	// }))

	debug("Listening to port: %s", port)
	http.ListenAndServe(":"+port, nil)
}
