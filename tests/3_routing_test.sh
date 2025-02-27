#!/usr/bin/env bash

cd ..
go build
cd client
go build
cd ..

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'
DEBUG="true"

outputFiles=()
message_c1_1=Weather_is_clear
message_c2_1=Winter_is_coming
message_c1_2=No_clouds_really
message_c2_2=Let\'s_go_skiing
message_c3=Is_anybody_here?


UIPort=12345
gossipPort=5000
name='A'

# General peerster (gossiper) command
#./Peerster -UIPort=12345 -gossipAddr=127.0.0.1:5001 -name=A -peers=127.0.0.1:5002 > A.out &

for i in `seq 1 10`;
do
	outFileName="$name.out"
	peerPort=$((($gossipPort+1)%10+5000))
	peer="127.0.0.1:$peerPort"
	gossipAddr="127.0.0.1:$gossipPort"
	./Peerster -UIPort=$UIPort -gossipAddr=$gossipAddr -name=$name -peers=$peer -rtimer=10 > "./tests/out/$outFileName" &
	outputFiles+=("./tests/out/$outFileName")
	if [[ "$DEBUG" == "true" ]] ; then
		echo "$name running at UIPort $UIPort and gossipPort $gossipPort"
	fi
	UIPort=$(($UIPort+1))
	gossipPort=$(($gossipPort+1))
	name=$(echo "$name" | tr "A-Y" "B-Z")
done


./client/client -UIPort=12349 -msg=$message_c1_1
./client/client -UIPort=12346 -msg=$message_c2_1
sleep 2
./client/client -UIPort=12349 -msg=$message_c1_2
sleep 1
./client/client -UIPort=12346 -msg=$message_c2_2
./client/client -UIPort=12351 -msg=$message_c3

sleep 5
pkill -f Peerster


#testing
failed="F"

echo -e "${RED}###CHECK that client messages arrived${NC}"

if !(grep -q "CLIENT MESSAGE $message_c1_1" "./tests/out/E.out") ; then
	failed="T"
fi

if !(grep -q "CLIENT MESSAGE $message_c1_2" "./tests/out/E.out") ; then
	failed="T"
fi

if !(grep -q "CLIENT MESSAGE $message_c2_1" "./tests/out/B.out") ; then
    failed="T"
fi

if !(grep -q "CLIENT MESSAGE $message_c2_2" "./tests/out/B.out") ; then
    failed="T"
fi

if !(grep -q "CLIENT MESSAGE $message_c3" "./tests/out/G.out") ; then
    failed="T"
fi

if [[ "$failed" == "T" ]] ; then
	echo -e "${RED}***FAILED***${NC}"
else
	echo -e "${GREEN}***PASSED***${NC}"
fi

failed="F"
echo -e "${RED}###CHECK rumor messages ${NC}"

gossipPort=5000
for i in `seq 0 9`;
do
	relayPort=$(($gossipPort-1))
	if [[ "$relayPort" == 4999 ]] ; then
		relayPort=5009
	fi
	nextPort=$((($gossipPort+1)%10+5000))
	msgLine1="RUMOR origin E from 127.0.0.1:[0-9]{4} ID [0-9]+ contents $message_c1_1"
	msgLine2="RUMOR origin E from 127.0.0.1:[0-9]{4} ID [0-9]+ contents $message_c1_2"
	msgLine3="RUMOR origin B from 127.0.0.1:[0-9]{4} ID [0-9]+ contents $message_c2_1"
	msgLine4="RUMOR origin B from 127.0.0.1:[0-9]{4} ID [0-9]+ contents $message_c2_2"
	msgLine5="RUMOR origin G from 127.0.0.1:[0-9]{4} ID [0-9]+ contents $message_c3"

	if [[ "$gossipPort" != 5004 ]] ; then
		if !(grep -Eq "$msgLine1" "${outputFiles[$i]}") ; then
        	failed="T"
			if [[ "$DEBUG" == "true" ]] ; then
		    	echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    	fi
    	fi
		if !(grep -Eq "$msgLine2" "${outputFiles[$i]}") ; then
        	failed="T"
			if [[ "$DEBUG" == "true" ]] ; then
		    	echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    	fi
    	fi
	fi

	if [[ "$gossipPort" != 5001 ]] ; then
		if !(grep -Eq "$msgLine3" "${outputFiles[$i]}") ; then
        	failed="T"
			if [[ "$DEBUG" == "true" ]] ; then
		    	echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    	fi
    	fi
		if !(grep -Eq "$msgLine4" "${outputFiles[$i]}") ; then
        	failed="T"
			if [[ "$DEBUG" == "true" ]] ; then
		    	echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    	fi
    	fi
	fi
	
	if [[ "$gossipPort" != 5006 ]] ; then
		if !(grep -Eq "$msgLine5" "${outputFiles[$i]}") ; then
        	failed="T"
			if [[ "$DEBUG" == "true" ]] ; then
		    	echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    	fi
    	fi
	fi
	gossipPort=$(($gossipPort+1))
done

if [[ "$failed" == "T" ]] ; then
    echo -e "${RED}***FAILED***${NC}"
else
    echo -e "${GREEN}***PASSED***${NC}"
fi


failed="F"
echo -e "${RED}###CHECK dsdv messages ${NC}"
gossipPort=5000
for i in `seq 0 9`;
do
    relayPort=$(($gossipPort-1))
    if [[ "$relayPort" == 4999 ]] ; then
        relayPort=5009
    fi
    nextPort=$((($gossipPort+1)%10+5000))

	msgLine1="DSDV E 127.0.0.1:\($relayPort\|$nextPort\)"
	msgLine2="DSDV B 127.0.0.1:\($relayPort\|$nextPort\)"
	msgLine3="DSDV G 127.0.0.1:\($relayPort\|$nextPort\)"

	if [[ ($gossipPort != 5004) && ( $(grep -c "$msgLine1" "${outputFiles[$i]}") < 2)]] ; then
        failed="T"
        if [[ "$DEBUG" == "true" ]] ; then
		    echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    fi
    fi
    if [[ ($gossipPort != 5001) && ( $(grep -c "$msgLine2" "${outputFiles[$i]}") < 2)]] ; then
        failed="T"
        if [[ "$DEBUG" == "true" ]] ; then
		    echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    fi
    fi
    if [[ ($gossipPort != 5006) && ( $(grep -c "$msgLine3" "${outputFiles[$i]}") < 1)]] ; then
        failed="T"
        if [[ "$DEBUG" == "true" ]] ; then
		    echo -e "${RED}Missing at ${outputFiles[$i]} ${NC}"
	    fi
    fi
	gossipPort=$(($gossipPort+1))
done

if [[ "$failed" == "T" ]] ; then
    echo -e "${RED}***FAILED***${NC}"
else
    echo -e "${GREEN}***PASSED***${NC}"
fi


failed="F"
echo -e "${RED}###CHECK correct peers${NC}"
gossipPort=5000
for i in `seq 0 9`;
do
    relayPort=$(($gossipPort-1))
    if [[ "$relayPort" == 4999 ]] ; then
        relayPort=5009
    fi
    nextPort=$((($gossipPort+1)%10+5000))

	peersLine1="127.0.0.1:$relayPort,127.0.0.1:$nextPort"
	peersLine2="127.0.0.1:$nextPort,127.0.0.1:$relayPort"

    if !(grep -q "$peersLine1" "${outputFiles[$i]}") && !(grep -q "$peersLine2" "${outputFiles[$i]}") ; then
        failed="T"
    fi
	gossipPort=$(($gossipPort+1))
done

if [[ "$failed" == "T" ]] ; then
    echo -e "${RED}***FAILED***${NC}"
else
    echo -e "${GREEN}***PASSED***${NC}"
fi

