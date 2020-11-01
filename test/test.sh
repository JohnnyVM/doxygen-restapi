#!/bin/sh

curl -v \
-H "Content-Type:application/tar" \
--data-binary "@./example_dir.tar.gz" \
localhost:8080/doxygen

curl -v \
-H "Content-Type:application/tar+gzip" \
--data-binary "@test/example_dir.tar.gz" \
localhost:8080/doxygen --output html.tar.gz

