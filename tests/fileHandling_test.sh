#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# We are building network like:
# a - b - c - d
# and then:
# sharing file at a -> downloading file at d from a -> downloading file at b from d

aUIPort=12001
bUIPort=12002
cUIPort=12003
dUIPort=12004

aPort=5001
bPort=5002
cPort=5003
dPort=5004

aAddr="127.0.0.1:$aPort"
bAddr="127.0.0.1:$bPort"
cAddr="127.0.0.1:$cPort"
dAddr="127.0.0.1:$dPort"

msgA1="Istartsharing!"
msgD1="Readytoreceive!"

sharedFileName="1M_file.txt"
downloadedFileName="downloaded-$sharedFileName"

cd ..
go build
cd client
go build
cd ..

./Peerster -name=a -peers="$bAddr"        -UIPort=$aUIPort -gossipAddr=$aAddr -rtimer=5 -antiEntropy=0  > "./tests/out/A.out" &
./Peerster -name=b -peers="$aAddr,$cAddr" -UIPort=$bUIPort -gossipAddr=$bAddr -rtimer=5 -antiEntropy=0 > "./tests/out/B.out" &
./Peerster -name=c -peers="$bAddr,$dAddr" -UIPort=$cUIPort -gossipAddr=$cAddr -rtimer=5 -antiEntropy=0 > "./tests/out/C.out" &
./Peerster -name=d -peers="$cAddr"        -UIPort=$dUIPort -gossipAddr=$dAddr -rtimer=5 -antiEntropy=0 > "./tests/out/D.out" &

# let the gossipers initialize
sleep 1

# some init messages, so a and d will know about each other
./client/client -UIPort="$aUIPort" -msg="LetsKnowEachOther1"
./client/client -UIPort="$dUIPort" -msg="LetsKnowEachOther2"
./client/client -UIPort="$aUIPort" -msg="LetsKnowEachOther3"
./client/client -UIPort="$dUIPort" -msg="LetsKnowEachOther4"

sleep 3
echo "~~~~~~ Initial rumor-mongering done, requesting file sharing ~~~~~~"

# prepare file to send:
cd _SharedFiles
rm "$sharedFileName"
dd if=/dev/urandom of="$sharedFileName"  bs=1MB  count=1 # copy a lot of nulls to file (a bit smaller, than 1M for technical reasons, haha)
echo "ahahha" >> "$sharedFileName"
cd ..
cd _Downloads
rm "b1-$downloadedFileName"
rm "b2-$downloadedFileName"
rm "d-$downloadedFileName" #"*$sharedFileName"
cd ..

# share big file on "a":
./client/client -UIPort="$aUIPort" -file="$sharedFileName"

sleep 2
fileHash=$(cat tests/out/A.out | grep "File $sharedFileName.*" | awk '{print $4}')

echo "~~~~~~ File shared, got hash: $fileHash, starting downloading ~~~~~~"

# request file on "d":
./client/client -UIPort="$dUIPort" -file="d-$downloadedFileName" -dest="a" -request="$fileHash"

sleep 10
echo "Downloading on d from a should have finished"

./client/client -UIPort="$dUIPort" -file="$sharedFileName"

sleep 1

# try to download file from "d" now: check if downloaded files are shared
./client/client -UIPort="$bUIPort" -file="b1-$downloadedFileName" -dest="d" -request="$fileHash"

sleep 10
echo "Downloading on b from d should have finished"

./client/client -UIPort="$bUIPort" -file="b2-$downloadedFileName" -dest="d" -request="$fileHash"

sleep 15
echo "Do downloading on b from d once again just for fun"

echo "Kill all the peerster processes..."
kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
sleep 1
echo "Killed"

echo "Tests running.."
if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/d-$downloadedFileName 2>&1)" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from a to d and saved at d as d-$downloadedFileName"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/d-$downloadedFileName 2>&1)"
fi

if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/b1-$downloadedFileName 2>&1)" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from d to b and saved at b as b-$downloadedFileName"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/b1-$downloadedFileName 2>&1)"
fi

if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/b2-$downloadedFileName 2>&1)" ]; then # err moved to out
    # if output of this command is empty
    echo -e "${GREEN}***PASSED***${NC}"
    echo "File $sharedFileName succesfully transfered from d to b and saved at b as b-$downloadedFileName"
else
    echo -e "${RED}***FAILED***${NC}"
    echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/b2-$downloadedFileName 2>&1)"
fi