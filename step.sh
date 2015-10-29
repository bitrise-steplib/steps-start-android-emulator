#!/bin/bash

set -e

if [ -z "${emulator_name}" ]; then
	printf "\e[31memulator_name was not specified\e[0m\n"
	exit 1
fi

ruby ./emulator.rb