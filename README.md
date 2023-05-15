# SFU: simple forwarding unit

## Protocol

### For receiving

```
url = protocol, host, path, query

protocol = 'ws', ['s'], '://'
 
hostname = <whatever valid character for a domain/IP address>, ':', { digit }

path = <whatever valid character for paths>

query = <whatever valid character for query>
```

The `query` MUST contain the following parameters:

- `keyid`
- `id`
- `kind`

All strings.

`kind` is especially important.

If the local RTCPeerConnection gets a track whose `kind` does not match `kind`, then the client can safely ignore it.