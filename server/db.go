package main

import (
	"encoding/json"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

func fetchNodeSeqLength(nodeId string) []byte {
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	// mongo query:
	// db.annot.findOne({id:"ENSAPLG00000013960"}, {length:1})
	resLim := struct {
		Length int `bson:"length" json:"limit"`
	}{}
	collection.Find(bson.M{"id": nodeId}).Select(bson.M{"length": 1, "_id": 0}).One(&resLim)

	j, err := json.Marshal(resLim)
	checkError("Problem marshaling json: ", err)

	return j
}

func fetchNode(nodeId string) *NodeDatum {
	db, err := mgo.Dial("mongodb://127.0.0.1:27017/")
	checkError("Problems connecting to mongodb server: ", err)
	defer db.Close()
	collection := db.DB("genetrees").C("annot")

	record := &NodeDatum{}
	collection.Find(bson.M{"id": nodeId}).Iter().Next(record)

	return record
}
