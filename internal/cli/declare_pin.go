package cli

import "github.com/donvargax/itos/v8/internal/declare"

var pinDeclared = declare.Command{
	Name: "pin",
	Rank: 120,
	Lines: `  pin [<version>|latest]           move the config's pin to a release, the newest by default
`,
	Help: map[string]string{
		"": `Usage: itos pin [<version>|latest]

Moves the config's pin to a release of itos: <version> (a leading v is taken),
or with none or latest the newest, asked for as the launcher asks for it
(<ITOS_RELEASES>/latest/download/checksums.txt) but now, whatever CI,
ITOS_NO_UPDATE or the once an hour say. It sets pin.version and pin.checksums,
the SHA-256 of that release's checksums.txt, in the config itos finds (the
stealth one included), adding the pin when there is none, and changes no other
byte of it; a layout it cannot edit so (a value over several lines, a tag, an
anchor) exits 2, the config untouched. It prints the old version, the new one
and the release's notes, <ITOS_RELEASES>/tag/v<version>, whose Upgrading
section says what the project must change, and commits nothing. A pin already
on the version changes nothing, exit 0; one on it with other checksums is left
as it is, exit 1: the release changed after it was pinned. A release the
server does not have exits 3, and a release server that cannot be reached or
fails 75, the config untouched. It runs the binary called, whatever the pin
says.

--json: {"schema":1,"config","action","version","checksums","previous"?,"notes"?},
        action one of pinned, already, refused`,
	},
}
