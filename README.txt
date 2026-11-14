This is basically a web proxy that "cleans up" the REST API implemented
on the DLI Pro Switch (https://dlidirect.com/products/new-pro-switch).  Note
that "cleaned up" is open for debate, and it supports a pile of other 
protocols, but this made me happy.  

Yes, I probably should have just used MQTT and been done with it, but here we 
are.

What this does is enable a fairly simple interface that can get and set current
power state.  It intentionally ignores transient states along with a ton of
other stuff.

Configuration lives in config.yml (or wherever --cfgpath points); see
example_config.yml.  The server has no authentication of its own, so it only
listens on 127.0.0.1 unless server.address says otherwise.

Outlets can be referred to by name or by number.  Numbers start at 1 and match
the labels on the hardware, even though the switch's own REST API counts from
0.  Names are whatever you've configured on the switch.  An unknown outlet
returns 400 Bad Request.

Examples:

  Get all outlets
  ---------------
    $ wget http://127.0.0.1:8000/api/outlets -O - --quiet | jq
    [
        {
            "index": 1,
            "name": "sprinkler",
            "on": false
        },
        {
            "index": 2,
            "name": "disco ball",
            "on": true
        },
        /* REDACTED */
    ]

  Get single outlet
  -----------------
    $ wget http://127.0.0.1:8000/api/outlets/2 -O - --quiet | jq
    {
        "index": 2,
        "name": "disco ball",
        "on": true
    }

  Get single outlet state
  -----------------------
    $ wget http://127.0.0.1:8000/api/outlets/2/state -O - --quiet
    on

    $ wget http://127.0.0.1:8000/api/outlets/sprinkler/state -O - --quiet
    off

  Set single outlet state
  -----------------------
  The body can be "on", "true", "off" or "false", in any case and with
  surrounding whitespace ignored.  Anything else returns 400 Bad Request.

    $ curl -X PUT http://127.0.0.1:8000/api/outlets/1/state --data "off"

    $ curl -X PUT http://127.0.0.1:8000/api/outlets/sprinkler/state --data "off"

  Get single outlet name
  ----------------------
    $ wget http://127.0.0.1:8000/api/outlets/2/name -O - --quiet
    disco ball

  Set single outlet name
  ----------------------
  The body is the new name, with surrounding whitespace ignored.  An empty
  name returns 400 Bad Request.  Avoid names that are plain numbers, since
  outlet numbers are matched before names.

    $ curl -X PUT http://127.0.0.1:8000/api/outlets/3/name --data "lava lamp"

    $ curl -X PUT "http://127.0.0.1:8000/api/outlets/lava%20lamp/name" --data "heater"
