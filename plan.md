### client-server protos

- stateUpdateMessage from client telling client id and change in position
- stateUpdateBroadcast broadcasting out change in state
- gameJoin from client to join game
- gameExit from client to leave game

### client-matchmaking protos

- availableForMatchmaking from client to matchmaking service to show availability to a game
- roomAddress from service to client when sufficient members have joined
- spawnRoom 