---
authors: Roman Tkachenko (roman@goteleport.com)
state: draft
---

# RFD 62 - Proxy matchers support for tsh proxy

## What

Proposes the UX for better `tsh proxy` command interoperability with `ssh`
client and connecting to leaf cluster proxies directly.

## Why

Proposed features are most useful for users who have many leaf clusters and
prefer to use plain `ssh` client to connect to nodes using jump hosts i.e.
connecting directly via leaf cluster's proxy rather than going through the
root cluster's proxy.

Consider the following scenario:

- User has multiple leaf clusters in different regions, for example
  `leaf1.us.acme.com`, `leaf2.eu.acme.com`, etc.
- Each leaf cluster has nodes like `node-1`, `node-2`, etc.
- User wants to log into a root cluster once to see the whole node inventory
  across all trusted clusters, but connect to the nodes within a particular
  leaf cluster through that cluster's proxy for better latency.

In order for the user to be able to, say, `ssh root@node-1.leaf1.us.acme.com`
currently they can create an SSH config similar to this:

```
Host *.leaf1.us.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@leaf1.us.acme.com -s proxy:$(echo %h | cut -d '.' -f1):%p@leaf1

Host *.leaf2.eu.acme.com
    HostName %h
    Port 3022
    ProxyCommand ssh -p 3023 %r@leaf2.eu.acme.com -s proxy:$(echo %h | cut -d '.' -f1):%p@leaf2
```

This is not ideal because users need to maintain complex SSH config, update it
every time a new leaf cluster is added, and use non-trivial bash logic in the
proxy command to correctly separate node name from the proxy address.

An ideal state would be where user's SSH config is as simple as:

```
Host *.acme.com
    HostName %h
    Port 3022
    ProxyCommand <some proxy command>
```

Where `<some proxy command>` is smart enough (and user-controllable) to
determine which host/proxy user is connecting to.

## UX

The proposal is to extend the existing `tsh proxy ssh` command with support for
parsing out the node name and proxy address from the full hostname token `%h` in
SSH config.

Specifically, the syntax for the `<some proxy command>` would look like:

```
Host *.acme.com
    HostName %h
    Port 3022
    ProxyCommand tsh proxy ssh --proxy=$proxy %r@%h:%p
```

With the `--proxy` flag set, the command connects directly to the specified
proxy instead of the default behavior of connecting to the proxy of the current
client profile.

When a templating variable `$proxy` is used, the host name and proxy address
are extracted from the full hostname in the `%r@%h:%p` spec.

Users define the rules of how to parse node/proxy from the full hostname in
the tsh config file `$TELEPORT_HOME/config/config.yaml`. Similar to role
templating, group captures are supported:

```yaml
proxy_matchers:
# Example matcher where nodes have short names like node-1, node-2, etc.
- matcher: "^(\w+)\.(leaf1.us.acme.com)$"
  host: "$1" # host is optional and will default to the full %h if not specified
  proxy: "$2:3023"
# Example matcher where nodes have FQDN names like node-1.leaf2.eu.acme.com.
- matcher: "^(\w+)\.(leaf2.eu.acme.com)$"
  proxy: "$2:443"
```

Matchers are evaluated in order and the first one matching will take effect.

In the example described above, where the user has nodes `node-1`, `node-2` in
multiple leaf clusters, their matchers configuration can look like:

```yaml
proxy_matchers:
- matcher: "^([^\.]+)\.(.+)$"
  host: "$1"
  proxy: "$2:3023"
```

Worth noting, that in the node spec `%r@%h:%p` the host name `%h` will be
replaced by the host from the matcher specification and will default to full
`%h` if it's not present in the matcher.

So given the above proxy matcher configuration, the following proxy command:

```bash
tsh proxy ssh --proxy=$proxy %r@%h:%p
```

is equivalent to the following when connecting to `node-1.leaf1.us.acme.com`:

```bash
tsh proxy ssh --proxy=leaf1.us.acme.com:3023 %r@node-1:3022
```
