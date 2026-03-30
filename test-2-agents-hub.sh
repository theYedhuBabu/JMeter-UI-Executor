#!/bin/bash

( cd jmeter-hub/ && ./jmeter-hub ) > hub.log 2>&1 &
echo "HUB IS RUNNING"

read -p "Do you want start two agents (y/n): " choice

if [ "$choice" = "y" ]; then


    ( cd jmeter-agent/ && ./jmeter-agent -hub="ws://localhost:8080/ws" ) > hub_2.log 2>&1 &
    echo "started Agent 1"

    sleep 5

    ( cd jmeter-agent/ && ./jmeter-agent -hub="ws://localhost:8080/ws" ) > hub_2.log 2>&1 &
    echo "Started Agent 2"
    
else
    echo "Okay, Agents Still running."
fi