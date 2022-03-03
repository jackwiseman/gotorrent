package main

func main() {
	link := "magnet:?xt=urn:btih:bdc0bb1499b1992a5488b4bbcfc9288c30793c08&tr=https%3A%2F%2Facademictorrents.com%2Fannounce.php&tr=udp%3A%2F%2Ftracker.coppersurfer.tk%3A6969&tr=udp%3A%2F%2Ftracker.opentrackr.org%3A1337%2Fannounce"

	torrent := new_torrent(link)

	torrent.find_peers()
	torrent.print_info()


	// now that we have our list of peers, lets attempt a handshake until we establish connection with one 

//	for i := 0; i < len(torrent.peers); i++ {
//		torrent.peers[i].connect()
//		torrent.peers[i].perform_handshake()
//		torrent.peers[i].disconnect()
//		if torrent.peers[i].is_alive {
//			return false
//		}
//	}
}
