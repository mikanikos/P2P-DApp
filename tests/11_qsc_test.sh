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

sharedFileName1="file1.txt"
sharedFileName2="file2.txt"
sharedFileName3="file3.txt"
sharedFileName4="file4.txt"
sharedFileName5="file5.txt"
sharedFileName6="file6.txt"
sharedFileName7="file7.txt"
sharedFileName8="file8.txt"
#downloadedFileName="downloaded-$sharedFileName"

cd ..
go build
cd client
go build
cd ..

./Peerster -name=a -peers="$bAddr"        -UIPort=$aUIPort -gossipAddr=$aAddr -rtimer=5 -antiEntropy=5 -hopLimit=5 -stubbornTimeout=10 -hw3ex2=true -hw3ex3=true -hw3ex4=true -N=4 > "./tests/out/A.out" &
./Peerster -name=b -peers="$aAddr,$cAddr" -UIPort=$bUIPort -gossipAddr=$bAddr -rtimer=5 -antiEntropy=5 -hopLimit=5 -stubbornTimeout=10 -hw3ex2=true -hw3ex3=true -hw3ex4=true -N=4 > "./tests/out/B.out" &
./Peerster -name=c -peers="$bAddr,$dAddr" -UIPort=$cUIPort -gossipAddr=$cAddr -rtimer=5 -antiEntropy=5 -hopLimit=5 -stubbornTimeout=10 -hw3ex2=true -hw3ex3=true -hw3ex4=true -N=4 > "./tests/out/C.out" &
./Peerster -name=d -peers="$cAddr"        -UIPort=$dUIPort -gossipAddr=$dAddr -rtimer=5 -antiEntropy=5 -hopLimit=5 -stubbornTimeout=10 -hw3ex2=true -hw3ex3=true -hw3ex4=true -N=4 > "./tests/out/D.out" &

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
rm "$sharedFileName1"
rm "$sharedFileName2"
rm "$sharedFileName3"
dd if=/dev/urandom of="$sharedFileName1"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName2"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName3"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName4"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName5"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName6"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName7"  bs=1MB  count=1
dd if=/dev/urandom of="$sharedFileName8"  bs=1MB  count=1
cd ..
# cd _Downloads
# rm "b1-$downloadedFileName"
# rm "b2-$downloadedFileName"
# rm "d-$downloadedFileName" #"*$sharedFileName"
# cd ..

# share big file on "a":
./client/client -UIPort="$aUIPort" -file="$sharedFileName1"

sleep 1

./client/client -UIPort="$bUIPort" -file="$sharedFileName2"

./client/client -UIPort="$dUIPort" -file="$sharedFileName3"

sleep 1

./client/client -UIPort="$cUIPort" -file="$sharedFileName4"

sleep 2

./client/client -UIPort="$aUIPort" -file="$sharedFileName5"

sleep 1

./client/client -UIPort="$bUIPort" -file="$sharedFileName6"

./client/client -UIPort="$dUIPort" -file="$sharedFileName7"

sleep 2

./client/client -UIPort="$cUIPort" -file="$sharedFileName8"

sleep 5

echo "Kill all the peerster processes..."
#kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
pkill -f Peerster
sleep 1
echo "Killed"

# echo "Tests running.."
# if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/d-$downloadedFileName 2>&1)" ]; then # err moved to out
#     # if output of this command is empty
#     echo -e "${GREEN}***PASSED***${NC}"
#     echo "File $sharedFileName succesfully transfered from a to d and saved at d as d-$downloadedFileName"
# else
#     echo -e "${RED}***FAILED***${NC}"
#     echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/d-$downloadedFileName 2>&1)"
# fi

# if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/b1-$downloadedFileName 2>&1)" ]; then # err moved to out
#     # if output of this command is empty
#     echo -e "${GREEN}***PASSED***${NC}"
#     echo "File $sharedFileName succesfully transfered from d to b and saved at b as b-$downloadedFileName"
# else
#     echo -e "${RED}***FAILED***${NC}"
#     echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/b1-$downloadedFileName 2>&1)"
# fi

# if [ -z "$(diff _SharedFiles/$sharedFileName _Downloads/b2-$downloadedFileName 2>&1)" ]; then # err moved to out
#     # if output of this command is empty
#     echo -e "${GREEN}***PASSED***${NC}"
#     echo "File $sharedFileName succesfully transfered from d to b and saved at b as b-$downloadedFileName"
# else
#     echo -e "${RED}***FAILED***${NC}"
#     echo "Bad output is: $(diff _SharedFiles/$sharedFileName _Downloads/b2-$downloadedFileName 2>&1)"
# fi