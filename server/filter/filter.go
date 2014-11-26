package main

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type LeafData map[string]*DbDatum
type GeneTrees map[string]LeafData

var (
	geneTrees GeneTrees
)

type Coord uint32

type Loc struct {
	From Coord `json:"from"`
	To   Coord `json:"to"`
}

type Gap struct {
	Start Coord  `bson:"start" json:"start"`
	End   Coord  `bson:"end" json:"end"`
	Type  string `bson:"type" json:"type"`
}
type Gaps []*Gap

type DbDatum struct {
	Id             string  `bson:"id"`
	Gt             string  `bson:"genetree"`
	Seq            string  `bson:"subseq"`
	ExonBoundaries []Coord `bson:"exon_boundaries"`
	Gaps           Gaps    `bson:"gaps"`
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	geneTrees = make(GeneTrees) // make(GeneTrees, 1000)
}

func getDbData(name string, gt LeafData, wg *sync.WaitGroup) {
	// mongo db connection
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	record := &DbDatum{}
	collection.Find(bson.M{"id": name}).Iter().Next(record)

	gt[name] = record
	wg.Done()
	return
}

func debug(format string, args ...interface{}) {
	log.Printf("DEBUG: "+format, args...)
}

func checkError(s string, err error, ss ...string) {
	if err != nil {
		log.Fatal(s, err, ss)
	}
}

func getAllNames(gt string) []string {
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	type names struct {
		Name string `bson:"id"`
	}
	ns := make([]names, 1000)
	collection.Find(bson.M{"genetree": gt}).Iter().All(&ns)

	var ids []string
	for _, n := range ns {
		if len(n.Name) != 0 {
			ids = append(ids, n.Name)
		}
	}

	return ids
}

func filterData(node *DbDatum, loc *Loc, filtData LeafData, wg *sync.WaitGroup) {
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
	var filtEBs []Coord
	for _, eb := range ebs {
		if (eb > loc.From) && (eb < loc.To) {
			filtEBs = append(filtEBs, eb)
		}
	}

	// Seq
	seq := node.Seq
	subseq := ""
	if len(seq) >= int(loc.To) {
		subseq = seq[loc.From:(loc.To - loc.From)]
	}

	filtData[node.Id] = &DbDatum{
		Id:             node.Id,
		Gt:             node.Gt,
		Seq:            subseq,
		ExonBoundaries: filtEBs,
		Gaps:           filtGaps}
	wg.Done()
}

func main() {
	gtId := "ENSGT00760000118818"

	if _, ok := geneTrees[gtId]; !ok {
		names := getAllNames(gtId)
		geneTree := make(LeafData, len(names))
		geneTrees[gtId] = geneTree

		var wg sync.WaitGroup
		wg.Add(len(names))
		for _, n := range names {
			go getDbData(n, geneTree, &wg)
		}
		wg.Wait()
	}

	loc := &Loc{From: 1000, To: 2000}
	fmt.Println(loc)
	l := len(geneTrees[gtId])
	fData := make(LeafData, l)
	var wg2 sync.WaitGroup
	wg2.Add(l)
	t1 := time.Now()
	for _, node := range geneTrees[gtId] {
		go filterData(node, loc, fData, &wg2)
	}
	wg2.Wait()
	t2 := time.Now()
	debug(fmt.Sprintf("Time to filter the tracks by loc: %v\n", t2.Sub(t1)))
	// fmt.Println(fData)
}
