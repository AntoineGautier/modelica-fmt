#!/bin/bash
set -e

go build .

for file in ./examples/*.mo; do
    if [[ $file == *-out.mo ]]; then
        continue
    fi
    filename=$(basename -- $file)
    outfile="${filename%.*}-out.mo"
    outfile80="${filename%.*}-80-out.mo"
    ./modelica-fmt $file > ./examples/${outfile}
    ./modelica-fmt -line-length 80 $file > ./examples/${outfile80}
done
