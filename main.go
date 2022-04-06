package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"unicode"

	"github.com/bingoohuang/gg/pkg/fla9"
	"github.com/bingoohuang/gg/pkg/iox"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/uid"

	"github.com/cockroachdb/pebble"
	"github.com/julienschmidt/httprouter"
)

func main() {
	port := 0
	fla9.IntVar(&port, "port,p", 8080, "Listen port")
	fla9.Parse()

	s, err := newServer("docdb.data", port)
	if err != nil {
		log.Fatal(err)
	}
	defer iox.Close(s.db)

	router := httprouter.New()
	router.POST("/docs", wrapHandler(s.addDocument))
	router.GET("/docs", wrapHandler(s.searchDocuments))
	router.GET("/docs/:id", wrapHandler(s.getDocument))

	log.Printf("Listening on %d", s.port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.port), router))
}

// H is alias for map[string]any.
type H map[string]any

func jsonResponseError(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusBadRequest)

	if err := json.NewEncoder(w).Encode(H{"status": "error", "error": err.Error()}); err != nil {
		log.Printf("encode json response failed: %v", err)
	}
}

func jsonResponse(w http.ResponseWriter, body H) error {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	if err := json.NewEncoder(w).Encode(H{"body": body, "status": "ok"}); err != nil {
		log.Printf("encode json response failed: %v", err)
	}
	return nil
}

type server struct {
	db      *pebble.DB // Primary data
	indexDb *pebble.DB // Index data
	port    int
}

func newServer(database string, port int) (s *server, err error) {
	s = &server{db: nil, port: port}
	if s.db, err = pebble.Open(database, &pebble.Options{}); err != nil {
		return nil, err
	}

	s.indexDb, err = pebble.Open(database+".index", &pebble.Options{})
	return
}

// Ignores arrays
func getPathValues(obj H, prefix string) (pvs []string) {
	for key, val := range obj {
		switch t := val.(type) {
		case H:
			pvs = append(pvs, getPathValues(t, key)...)
			continue
		case []interface{}: // Can't handle arrays
			continue
		}

		if prefix != "" {
			key = prefix + "." + key
		}

		pvs = append(pvs, fmt.Sprintf("%s=%v", key, val))
	}

	return pvs
}

func (s server) index(id string, document H) {
	pv := getPathValues(document, "")

	for _, pathValue := range pv {
		idsString, closer, err := s.indexDb.Get([]byte(pathValue))
		if err != nil && err != pebble.ErrNotFound {
			log.Printf("Could not look up pathvalue [%#v]: %s", document, err)
		}

		if len(idsString) == 0 {
			idsString = []byte(id)
		} else {
			ids := strings.Split(string(idsString), ",")
			if found := ss.AnyOf(id, ids...); !found {
				idsString = append(idsString, []byte(","+id)...)
			}
		}

		if closer != nil {
			iox.Close(closer)
		}
		if err = s.indexDb.Set([]byte(pathValue), idsString, pebble.Sync); err != nil {
			log.Printf("Could not update index: %s", err)
		}
	}
}

func (s server) addDocument(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	var doc H
	if err := json.NewDecoder(r.Body).Decode(&doc); err != nil {
		return err
	}

	id := uid.New().String() // New ksuid for the document
	s.index(id, doc)

	bs, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	if err := s.db.Set([]byte(id), bs, pebble.Sync); err != nil {
		return err
	}

	return jsonResponse(w, H{"id": id})
}

type queryComparison struct {
	key   []string
	value string
	op    string
}

type query struct {
	ands []queryComparison
}

func getPath(doc H, parts []string) (any, bool) {
	var docSegment any = doc
	for _, part := range parts {
		m, ok := docSegment.(H)
		if !ok {
			return nil, false
		}

		if docSegment, ok = m[part]; !ok {
			return nil, false
		}
	}

	return docSegment, true
}

func (q query) match(doc H) bool {
	for _, argument := range q.ands {
		value, ok := getPath(doc, argument.key)
		if !ok {
			return false
		}

		// Handle equality
		if argument.op == "=" {
			if match := fmt.Sprintf("%v", value) == argument.value; !match {
				return false
			}

			continue
		}

		// Handle <, >
		right, err := strconv.ParseFloat(argument.value, 64)
		if err != nil {
			return false
		}

		var left float64
		switch t := value.(type) {
		case float32, float64:
			left = reflect.ValueOf(value).Float()
		case uint, uint8, uint16, uint32, uint64:
			left = float64(reflect.ValueOf(value).Uint())
		case int, int8, int16, int32, int64:
			left = float64(reflect.ValueOf(value).Int())
		case string:
			if left, err = strconv.ParseFloat(t, 64); err != nil {
				return false
			}
		default:
			return false
		}

		if argument.op == ">" {
			if left <= right {
				return false
			}

			continue
		}

		if left >= right {
			return false
		}
	}

	return true
}

// Handles either quoted strings or unquoted strings of only contiguous digits and letters
func lexString(input []rune, index int) (string, int, error) {
	if index >= len(input) {
		return "", index, nil
	}
	if input[index] == '"' {
		index++
		foundEnd := false

		var s []rune
		// TODO: handle nested quotes
		for index < len(input) {
			if input[index] == '"' {
				foundEnd = true
				break
			}

			s = append(s, input[index])
			index++
		}

		if !foundEnd {
			return "", index, fmt.Errorf("expected end of quoted string")
		}

		return string(s), index + 1, nil
	}

	// If unquoted, read as much contiguous digits/letters as there are
	var s []rune
	var c rune
	// TODO: someone needs to validate there's not ...
	for index < len(input) {
		c = input[index]
		if !(unicode.IsLetter(c) || unicode.IsDigit(c) || c == '.') {
			break
		}
		s = append(s, c)
		index++
	}

	if len(s) == 0 {
		return "", index, fmt.Errorf("no string found")
	}

	return string(s), index, nil
}

// E.g. q=a.b:12
func parseQuery(q string) (*query, error) {
	if q == "" {
		return &query{}, nil
	}

	var parsed query
	qRune := []rune(q)
	for i := 0; i < len(qRune); {
		// Eat whitespace
		for unicode.IsSpace(qRune[i]) {
			i++
		}

		key, nextIndex, err := lexString(qRune, i)
		if err != nil {
			return nil, fmt.Errorf("expected valid key, got [%s]: `%s`", err, q[nextIndex:])
		}

		if q[nextIndex] != ':' {
			return nil, fmt.Errorf("expected colon at %d, got: `%s`", nextIndex, q[nextIndex:])
		}
		i = nextIndex + 1

		op := "="
		if q[i] == '>' || q[i] == '<' {
			i++
			op = string(q[i])
		}

		value, nextIndex, err := lexString(qRune, i)
		if err != nil {
			return nil, fmt.Errorf("expected valid value, got [%s]: `%s`", err, q[nextIndex:])
		}
		i = nextIndex

		argument := queryComparison{key: strings.Split(key, "."), value: value, op: op}
		parsed.ands = append(parsed.ands, argument)
	}

	return &parsed, nil
}

func (s server) getDocumentByID(id []byte) (H, error) {
	valBytes, closer, err := s.db.Get(id)
	if err != nil {
		return nil, err
	}
	defer iox.Close(closer)

	var document H
	err = json.Unmarshal(valBytes, &document)
	return document, err
}

func (s server) lookup(pathValue string) ([]string, error) {
	idsString, closer, err := s.indexDb.Get([]byte(pathValue))
	if err != nil && err != pebble.ErrNotFound {
		return nil, fmt.Errorf("could not look up pathvalue [%#v]: %s", pathValue, err)
	}
	defer iox.Close(closer)

	if len(idsString) == 0 {
		return nil, nil
	}

	return strings.Split(string(idsString), ","), nil
}

func wrapHandler(h func(http.ResponseWriter, *http.Request, httprouter.Params) error) func(http.ResponseWriter, *http.Request, httprouter.Params) {
	return func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		if err := h(w, r, p); err != nil {
			jsonResponseError(w, err)
		}
	}
}
func (s server) searchDocuments(w http.ResponseWriter, r *http.Request, _ httprouter.Params) error {
	q, err := parseQuery(r.URL.Query().Get("q"))
	if err != nil {
		return err
	}

	isRange := false
	idsArgumentCount := map[string]int{}
	nonRangeArguments := 0
	for _, argument := range q.ands {
		if argument.op == "=" {
			nonRangeArguments++

			ids, err := s.lookup(fmt.Sprintf("%s=%v", strings.Join(argument.key, "."), argument.value))
			if err != nil {
				return err
			}

			for _, id := range ids {
				if _, ok := idsArgumentCount[id]; !ok {
					idsArgumentCount[id] = 0
				}

				idsArgumentCount[id]++
			}
		} else {
			isRange = true
		}
	}

	var idsInAll []string
	for id, count := range idsArgumentCount {
		if count == nonRangeArguments {
			idsInAll = append(idsInAll, id)
		}
	}

	var documents []any
	if r.URL.Query().Get("skipIndex") == "true" {
		idsInAll = nil
	}
	if len(idsInAll) > 0 {
		for _, id := range idsInAll {
			doc, err := s.getDocumentByID([]byte(id))
			if err != nil {
				return err
			}

			if !isRange || q.match(doc) {
			}
		}
	} else {
		iter := s.db.NewIter(nil)
		defer iox.Close(iter)
		for iter.First(); iter.Valid(); iter.Next() {
			var doc H
			if err := json.Unmarshal(iter.Value(), &doc); err != nil {
				return err
			}

			if q.match(doc) {
				documents = append(documents, H{"id": string(iter.Key()), "body": doc})
			}
		}
	}

	return jsonResponse(w, H{"documents": documents, "count": len(documents)})
}

func (s server) getDocument(w http.ResponseWriter, _ *http.Request, ps httprouter.Params) error {
	id := ps.ByName("id")
	doc, err := s.getDocumentByID([]byte(id))
	if err != nil {
		return err
	}

	return jsonResponse(w, H{"document": doc})
}

func (s server) reindex() {
	iter := s.db.NewIter(nil)
	defer iox.Close(iter)
	for iter.First(); iter.Valid(); iter.Next() {
		var doc H
		if err := json.Unmarshal(iter.Value(), &doc); err != nil {
			log.Printf("Unable to parse bad document, %s: %s", string(iter.Key()), err)
		}
		s.index(string(iter.Key()), doc)
	}
}
