package main

//Calendar feed structs
type calEntry struct {
	Title string
	Content string
}

type calFeed struct {
	TotalResults int
	Entry []calEntry
}

//Dictionary structs
type dictWordnet struct {
	Pos []dictPos
}

type dictPos struct {
	Name string "attr"
	Category dictCategory
}

type dictCategory struct {
	Sense []dictSense
}

type dictSense struct{
	Synset struct {Word []string; Definition, Sample string}
}

//Wiki struct
type wiki struct {
	Page struct {
		Title string;
		Body struct{Text string}
	}
}
