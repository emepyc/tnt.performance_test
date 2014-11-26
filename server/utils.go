package main

import (
	"fmt"
	"log"
)

func debug(format string, args ...interface{}) {
	log.Printf("DEBUG: "+format, args...)
}

func checkError(s string, err error, ss ...string) {
	if err != nil {
		log.Fatal(s, err, ss)
	}
}

func (t Track) String() string {
	s := fmt.Sprintf("   NAME   : %s\n", t.Name)
	s += fmt.Sprintf("   HEIGHT : %d\n", t.Height)
	s += fmt.Sprintf("   V_OFFS : %d\n", t.VOffset)
	return s
}

func (t Tracks) String() string {
	s := "TRACKS:\n"
	for _, t := range t {
		s += fmt.Sprintf("%s", t)
	}
	return s
}

func (l Loc) String() string {
	s := "LOCATION\n"
	s += fmt.Sprintf("  FROM : %d\n", l.From)
	s += fmt.Sprintf("  TO   : %d\n", l.To)
	return s
}

func (b Color) String() string {
	s := fmt.Sprintf("       R:%d\n", b.R)
	s += fmt.Sprintf("       G:%d\n", b.G)
	s += fmt.Sprintf("       B:%d\n", b.B)
	return s
}

func (c Conf) String() string {
	s := "CONFIG\n"
	s += fmt.Sprintf("  HEIGHT  : %d\n", c.Height)
	s += fmt.Sprintf("  WIDTH   : %d\n", c.Width)
	s += fmt.Sprintf("  BGCOLOR : \n%s\n", c.BgColor)
	return s
}

func (p *PostData) String() string {
	s := fmt.Sprintf("%s\n", p.Loc)
	s += fmt.Sprintf("%s\n", p.Conf)
	s += fmt.Sprintf("%s\n", p.Tracks)
	return s
}

func (g Gap) String() string {
	s := fmt.Sprintf("(%d:%d[%s])", g.Start, g.End, g.Type)
	return s
}

func (gs Gaps) String() string {
	s := ""
	for _, g := range gs {
		s += fmt.Sprintf("%s ", g)
	}
	return s
}

func (d DbDatum) String() string {
	s := fmt.Sprintf("ID: %s\nSEQ: %s\nLEN: %d\nGAPS: %s\nEXONS:%s\n", d.Id, d.Seq, len(d.Seq), d.Gaps, d.ExonBoundaries)
	return s
}

// func (d DbDatum) String() string {
// 	s := fmt.Sprintf("%s (%d:%d) ", d.Name, d.Start, d.End)
// 	return s
// }
