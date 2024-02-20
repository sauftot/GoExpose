# GoExpose
Simple reverse proxy to expose a TCP application server behind uncontrollable NAT to the internet.<br>
It consists of 2 applications, one running on your local server and one running on a cheap, rented VPS or any other node exposed and routable from the internet.<br><br>
RPserver and RPclient are both tested on ubuntu 22.04 with minecraft as the exposed application server.

## Rewrite
Both the server and the client are rewritten, and currently in testing status. The old RP project was moved to legacy. New rewrites implement TLS authentication and encryption for communication between server and client, proper logging through a self-written small logger wrapper package, more efficient proxying and better synchronization using contexts and other smart golang features. Planned for this rewrite is implementing extensive unit tests, as well as automated deployment if feasible.

## Firewall integration
New Issues and Branches have been created to implement firewall manipulation by the server application. It will be able to add and delete rules to both the Service Providers External Firewall through a custom external module using its API, as well as the internal OS Firewall.
