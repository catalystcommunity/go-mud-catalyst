# muddycore

The Catalyst Community MUD library, made in Go

This is an experimental core of a MUD meant to be a nice hacking project for a break between more serious projects. It was started by TodPunk as a palate cleanser. The only real goal is to start working towards a simple world that people could chat in, but with support for modern things like being accessible through a web interface through a web socket and getting chat history.

## Current State

We're just getting started. At the moment we're going for a generic message handling and using [CBOR](https://cbor.io/) for the encoding since it's a nice standard binary protocol we don't have to manage types for, but the core messages inside those can be any type as it's just byte array payloads. Convenience and performance.

Prototyping is good. If you are interested in talking about this or other projects, we have a [Discord](https://discord.gg/jhUgW6zmzg)

## Organization

None. None organization.

Our current thinking is that the pkg dir has all our library stuff which should have all the building blocks for a MUD and will run a server and give client stuff and all that.

## Contributing

First open an issue with:

- What you want to do (your feature)
- What your plan to do it is (or none, and we can talk about approaches)
- What your general timeline might be to implement it. You won't be held to the time, but it lets us plan a bit or close things out if something goes wrong.

Yes, this is about you doing the work for your request. That's the point of this thing. We will not accept plain feature requests, and we will not accept PRs where a discussion has not happened prior to the PR being opened.

Have at it! (When we have code to have at, of course)

