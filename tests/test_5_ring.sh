#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

# We are building network like:
# a - b - c - d
# and then doing rumor & private messages on it to test the system

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

msgA1="WhyAreYouSoRight?"
msgD1="BecauseIDontLikeCommies!"
msgA2="ButHelpingOtherIsGood!"
msgD2="AreYouSure?ICareOnlyAboutMyself"
msgC1="StopArguingGuys!"
msgB1="CIsRight,CentristsAreBest"
msgA3="WeWillStillMakeRevolutionInYourLands"
msgD3="Try"

cd ..
go build
cd client
go build
cd ..

./Peerster -name=a -peers="$bAddr"        -UIPort=$aUIPort -gossipAddr=$aAddr -rtimer=0 -antiEntropy=0  > "./tests/out/A.out" &
./Peerster -name=b -peers="$aAddr,$cAddr" -UIPort=$bUIPort -gossipAddr=$bAddr -rtimer=0 -antiEntropy=0 > "./tests/out/B.out" &
./Peerster -name=c -peers="$bAddr,$dAddr" -UIPort=$cUIPort -gossipAddr=$cAddr -rtimer=0 -antiEntropy=0 > "./tests/out/C.out" &
./Peerster -name=d -peers="$cAddr"        -UIPort=$dUIPort -gossipAddr=$dAddr -rtimer=0 -antiEntropy=0 > "./tests/out/D.out" &

# let the gossipers initialize
sleep 1

# some init messages, so a and d will know about each other (with only rtimer initalization prob, that they know each other equals 2*(9/16)-(9/16)**2~0.8
./client/client -UIPort=$aUIPort -msg="LetsKnowEachOther1"
./client/client -UIPort=$dUIPort -msg="LetsKnowEachOther2"
./client/client -UIPort=$aUIPort -msg="LetsKnowEachOther3"
./client/client -UIPort=$dUIPort -msg="LetsKnowEachOther4"
# private msgs:
./client/client -UIPort=$aUIPort -dest="d" -msg=$msgA1
./client/client -UIPort=$dUIPort -dest="a" -msg=$msgD1
./client/client -UIPort=$aUIPort -dest="d" -msg=$msgA2
./client/client -UIPort=$dUIPort -dest="a" -msg=$msgD2
# rumors:
./client/client -UIPort=$cUIPort -msg=$msgC1
./client/client -UIPort=$bUIPort -msg=$msgB1
# private msgs:
./client/client -UIPort=$aUIPort -dest="d" -msg=$msgA3
./client/client -UIPort=$dUIPort -dest="a" -msg=$msgD3
# final rumors:
./client/client -UIPort=$aUIPort -msg="LetsKnowEachOther5"
./client/client -UIPort=$dUIPort -msg="LetsKnowEachOther6"


# let gossipers work for some time
sleep 5
echo "Kill all the peerster processes..."
kill $(ps aux | grep '\.\/[P]eerster' | awk '{print $2}')
sleep 1
echo "Killed"

# start testing:
echo -e "${RED}###CHECK that rumor messages arrived${NC}"
failed="F"

if !(grep -q "RUMOR origin b .* $msgB1" "./tests/out/A.out") ; then
        echo "failed11"
        failed="T"
fi
if !(grep -q "RUMOR origin b .* $msgB1" "./tests/out/C.out") ; then
        echo "failed12"
        failed="T"
fi
if !(grep -q "RUMOR origin c .* $msgC1" "./tests/out/A.out") ; then
        echo "failed13"
        failed="T"
fi
if !(grep -q "RUMOR origin c .* $msgC1" "./tests/out/D.out") ; then
        echo "failed14"
        failed="T"
fi
if [[ "$failed" == "T" ]] ; then
        echo -e "${RED}***FAILED***${NC}"
else
        echo -e "${GREEN}***PASSED***${NC}"
fi

echo -e "${RED}###CHECK that private messages arrived${NC}"
failed="F"

if !(grep -q "PRIVATE origin d hop-limit [0-9]* contents $msgD1" "./tests/out/A.out") ; then
        echo "failed21"
        failed="T"
fi
if !(grep -q "PRIVATE origin d hop-limit [0-9]* contents $msgD3" "./tests/out/A.out") ; then
        echo "failed22"
        failed="T"
fi
if !(grep -q "PRIVATE origin a hop-limit [0-9]* contents $msgA2" "./tests/out/D.out") ; then
        echo "failed23"
        failed="T"
fi
if !(grep -q "PRIVATE origin a hop-limit [0-9]* contents $msgA3" "./tests/out/D.out") ; then
        echo "failed24"
        failed="T"
fi

if [[ "$failed" == "T" ]] ; then
        echo -e "${RED}***FAILED***${NC}"
else
        echo -e "${GREEN}***PASSED***${NC}"
fi

