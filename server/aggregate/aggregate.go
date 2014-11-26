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
	Id             string  `bson:"_id"`
	Seq            string  `bson:"subseq"`
	ExonBoundaries []Coord `bson:"exon_boundaries"`
	Gaps           Gaps    `bson:"gaps"`
	// Length         uint32  `bson:"length"`
	// nExonBounds    uint32  `bson:"nexon_bounds"`
	// nGaps          uint32  `bson:"ngaps"`
}

func init() {
	runtime.GOMAXPROCS(runtime.NumCPU())
}

//func getDbData(l *Loc, name string, collection *mgo.Collection, wg *sync.WaitGroup) {
func getDbData(l *Loc, name string, wg *sync.WaitGroup) {
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	matchID := bson.M{"$match": bson.M{"id": name}}
	// unwGaps := bson.M{"$unwind": "$gaps"}
	// unwEB := bson.M{"$unwind": "$exon_boundaries"}
	// matchRange := bson.M{"$gte": l.From, "$lte": l.To}
	// gapsStart := bson.M{"gaps.start": matchRange}
	// gapsEnd := bson.M{"gaps.end": matchRange}
	// or := []bson.M{gapsStart, gapsEnd}
	// matchGaps := bson.M{"$match": bson.M{"$or": or}}
	// matchEB := bson.M{"$match": bson.M{"exon_boundaries": matchRange}}
	// project := bson.M{"$project": bson.M{
	// 	"id":              1,
	// 	"gaps":            1,
	// 	"exon_boundaries": 1,
	// 	"subseq": bson.M{"$substr": []interface{}{
	// 		"$seq",
	// 		l.From,
	// 		(l.To - l.From)}}}}
	// group := bson.M{"$group": bson.M{
	// 	"_id":             "$id",
	// 	"gaps":            bson.M{"$addToSet": "$gaps"},
	// 	"exon_boundaries": bson.M{"$addToSet": "$exon_boundaries"},
	// 	"subseq":          bson.M{"$first": "$subseq"}}}

	pipeline := []bson.M{
		matchID}
	// unwGaps,
	// unwEB,
	// matchGaps,
	// matchEB,
	// project,
	// group}

	dbT1 := time.Now()
	pipe := collection.Pipe(pipeline)
	iter := pipe.Iter()
	record := &DbDatum{}
	for iter.Next(&record) {
		// fmt.Println("==>", record, "<===")
	}
	if err := iter.Close(); err != nil {
		checkError("Closing iterator... ", err)
	}
	dbT2 := time.Now()
	wg.Done()
	debug(fmt.Sprintf("Time to retrieve %s track data from db: %v\n", name, dbT2.Sub(dbT1)))
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

func getAllNames(collection *mgo.Collection) []string {
	type names struct {
		Name string `bson:"id"`
	}
	ns := make([]names, 1000)
	collection.Find(bson.M{}).Iter().All(&ns)

	var ids []string
	for _, n := range ns {
		if len(n.Name) != 0 {
			ids = append(ids, n.Name)
		}
	}

	return ids
}

func main() {
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")
	names := getAllNames(collection)

	var wg sync.WaitGroup
	wg.Add(len(names))
	loc := &Loc{From: 1000, To: 2000}
	for _, n := range names {
		go getDbData(loc, n, &wg)
	}
	wg.Wait()
	// getDbData(loc, "ENSTBEG00000012937", collection)
}
