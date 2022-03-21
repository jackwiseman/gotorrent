<div align="center">
	<h1>GoTorrent</h1>
	<p>A CLI Bittorrent Client written in GoLang</p>
	<p>Note: This project is currently *heavily* under development and not intended to be used -- yet</p>
</div>


### Features
 - Magnet link support (.torrent files are not currently planned)
 - Scrapes torrent info, displaying # of seeders/leechers
 - Fetches metadata from magnet links' embedded trackers

### Planned Features
 - Single file downloads
 - Multi-file downloads

### Motivation
With BitTorrent remaining the single largest file-sharing protocol since its initial release in 2001, I thought it might be interesting to explore exactly how the protocol works. In order to implement thus far, I've utilized the (somewhat outdated) [WikiTheory Documentation](https://wiki.theory.org/BitTorrentSpecification) along with the BitTorrent-published [BEPs](http://www.bittorrent.org/beps/bep_0000.html) (**B**itTorrent **E**nhancement **P**roposals). Most of what I have been able to implement thus far is leech-heavy, I don't anticipate writing a client meant to be left open for long periods of time, but mainly focused on downloading the contents of torrents pointed to by magnet links. Besides learning about the protocol itself, I thought it would be interresting to build upon what I learned for my [EncryptedChat](http://www.github.com/jackwiseman/encryptedchat) project and work with a network protocol that is actually utilized today.
