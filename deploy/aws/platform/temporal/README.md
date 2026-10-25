# Maintained Temporal binaries

Build the same server 1.32.0 and CLI 1.9.0 sources as the pinned upstream images,
using the reviewed Go compiler and fixed dependencies locked in this module.
The CLI extension uses the matching upstream commit through the public Go module
proxy. Go checksums and `go mod verify` cover source downloads.
Server tools and the CLI have separate dependency graphs so their existing SDK
versions are retained independently.

The final images inherit the original user, commands, configuration and SQL schema
files. Only the existing executable paths and accompanying license notices are
copied into them. Build dependency packages directly so original module versions
remain visible to vulnerability scanners. The full input findings, replacement
inventories and strict output scans remain required.

A server or CLI version change requires its own compatibility and schema review;
these dependency rebuilds do not upgrade Temporal database schemas.
