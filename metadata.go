package main

type Metadata struct {
	Name string "name"
	Name_utf string "name.utf-8"
	Piece_len int "piece length"
	Pieces string "pieces"
	// contains one of the following, where 'length' means there is one file, and 'files' means there are multiple, only single file downloads will be allowed for the moment
	Length int "length"
	Files int "files"
}
