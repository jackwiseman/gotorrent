package main

func main() {
	// Magnet links taken from AcademicTorrents.com
	link := "magnet:?xt=urn:btih:8c271f4d2e92a3449e2d1bde633cd49f64af888f&tr=https%3A%2F%2Facademictorrents.com%2Fannounce.php&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce"
//	link := "magnet:?xt=urn:btih:bdc0bb1499b1992a5488b4bbcfc9288c30793c08&tr=https%3A%2F%2Facademictorrents.com%2Fannounce.php&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce"
	torrent := new_torrent(link)

//	torrent.find_peers()

//	torrent.get_metadata()
	torrent.parse_metadata_file()
	torrent.print_info()

}
