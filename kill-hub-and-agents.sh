#!/bin/bash

killall jmeter-hub 

echo "Killed HUB"

read -p "Do you want kill all jmeter-agents? (y/n): " choice

if [ "$choice" = "y" ]; then


    killall jmeter-agent
    echo "Killed all agents"
    
else
    echo "Okay, Agents Still running."
fi

