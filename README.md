# jitsi-peephole-service

A simple service which exposes the
[mod_muc_census](https://github.com/jitsi/jitsi-meet/blob/6682b52a1947deb0cf28043d3816b1081ef84c2b/resources/prosody-plugins/mod_muc_census.lua)
Prosody room statistics for one single room (configured via `PEEPHOLE_ROOM_NAME`).

## Environment variables

- `PEEPHOLE_ROOM_NAME` _(required)_: Room name for which statistics are exposed (example: `foobar@muc.meet.jitsi`)
- `PEEPHOLE_HTTP_ADDR` _(default `:9339`)_: Address on which the peephole HTTP server will listen on
- `XMPP_SERVER` _(default `xmpp.meet.jitsi`)_: Hostname of the Prosody HTTP service
- `PROSODY_HTTP_PORT` _(default `5280`)_: Port of the Prosody HTTP service
