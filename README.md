# P2P DApp
Peer-to-peer decentralized application in Go, it includes:
 
- gossip protocol (rumormongering, anti-entropy);
- routing protocol (DSDV);
- file handling (share, upload, download, search file chunks);
- consensus (TLC and Que-Sera-Consensus) and blockchain on top of the file handler to agree on the files in the network;
- CLI client;
- webserver and simple UI to issue commands and use the main functionalities.

## Usage
Simply compile (`go build`) and run the [client](./client) and the [server](./); use `--help` to see all the options of the two commands.
