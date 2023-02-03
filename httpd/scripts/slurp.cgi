#!/bin/bash -e
#
# Copyright (c) 2021-2023 Apple Inc. Licensed under MIT License.

echo "Content-type: text/html"
echo ""

echo '<html><head><meta http-equiv="Content-Type" content="text/html; charset=UTF-8"><title></title></head>'
echo '<body>'
if [ "$REQUEST_METHOD" = "POST" ]; then
    echo "<p>POST</p>"
    if [ "$CONTENT_LENGTH" -gt 0 ]; then
        echo "CL: $CONTENT_LENGTH"
    fi
fi
echo '</body></html>'

exit 0
