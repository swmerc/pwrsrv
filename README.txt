This is basically a web proxy that "cleans up" the REST API implemented
on the DLI Pro Switch (https://dlidirect.com/products/new-pro-switch).  Note
that "cleaned up" is open for debate, and it supports a pile of other 
protocols, but this made me happy.  

Yes, I probably should have just used MQTT and been done with it, but here we 
are.

What this does is enable a fairly simple interface that can get and set current
power state.  It intentionally ignores transient states along with a ton of
other stuff. Examples:

  Get all outlets
  ---------------
    $ wget http://127.0.0.1:8000/api/outlets -O - --quiet | jq
    [
        {
            "name": "sprinkler",
            "on": false
        },
        {
            "name": "disco ball",
            "on": false
        },
        /* REDACTED */
    ]

  Get single outlet
  -----------------
    $ wget http://127.0.0.1:8000/api/outlets/1 -O - --quiet | jq
    {
        "name": "disco ball",
        "on": true
    }

  Get single outlet state
  -----------------
    $ wget http://127.0.0.1:8000/api/outlets/1/state -O - --quiet
    true

  Set single outlet state
  -----------------------
    $ curl -X PUT http://127.0.0.1:8000/api/outlets/1/state --data "off"

