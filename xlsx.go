package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// xlsxToCSV reads an Excel workbook (such as the one Microsoft Forms
// exports) and returns the sheet that looks like the response log as CSV.
// Date cells come through as their Excel serial numbers; parseWhen reads those.
func xlsxToCSV(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("That does not look like an Excel file.")
	}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	read := func(name string) []byte {
		f := files[name]
		if f == nil {
			return nil
		}
		rc, err := f.Open()
		if err != nil {
			return nil
		}
		defer rc.Close()
		b, _ := io.ReadAll(io.LimitReader(rc, 60<<20))
		return b
	}
	var sst struct {
		SI []struct {
			T string `xml:"t"`
			R []struct {
				T string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	xml.Unmarshal(read("xl/sharedStrings.xml"), &sst)
	shared := make([]string, len(sst.SI))
	for i, si := range sst.SI {
		shared[i] = si.T
		for _, r := range si.R {
			shared[i] += r.T
		}
	}
	best, bestScore := "", -1
	for i := 1; i <= 40; i++ {
		b := read(fmt.Sprintf("xl/worksheets/sheet%d.xml", i))
		if b == nil {
			continue
		}
		rows := parseSheet(b, shared)
		score := len(rows)
		for _, r := range rows[:min(len(rows), 8)] {
			for _, c := range r {
				lc := strings.ToLower(c)
				if lc == "start time" || lc == "completion time" || lc == "timestamp" {
					score += 100000
				}
			}
		}
		if score > bestScore {
			var sb strings.Builder
			w := csv.NewWriter(&sb)
			w.WriteAll(rows)
			best, bestScore = sb.String(), score
		}
	}
	if best == "" {
		return "", fmt.Errorf("No sheets were found in that Excel file.")
	}
	return best, nil
}

func colIndex(ref string) int {
	n := 0
	for _, ch := range ref {
		if ch < 'A' || ch > 'Z' {
			break
		}
		n = n*26 + int(ch-'A') + 1
	}
	return n - 1
}

func parseSheet(b []byte, shared []string) [][]string {
	var ws struct {
		Rows []struct {
			Cells []struct {
				R  string `xml:"r,attr"`
				T  string `xml:"t,attr"`
				V  string `xml:"v"`
				IS struct {
					T string `xml:"t"`
				} `xml:"is"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	xml.Unmarshal(b, &ws)
	var out [][]string
	for _, r := range ws.Rows {
		var row []string
		for _, c := range r.Cells {
			i := colIndex(c.R)
			for len(row) <= i {
				row = append(row, "")
			}
			v := c.V
			switch c.T {
			case "s":
				if n, err := strconv.Atoi(v); err == nil && n < len(shared) {
					v = shared[n]
				}
			case "inlineStr":
				v = c.IS.T
			}
			row[i] = v
		}
		out = append(out, row)
	}
	return out
}
