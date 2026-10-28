#!/bin/sh
# Only the five certificates v0.30.2 compatibility edits in Caddy's runtime.
# Run after go mod download && go mod verify, in the disposable Caddy builder.
set -eu

module=github.com/smallstep/certificates
test "$(go list -m -f '{{.Version}}' "$module")" = v0.30.2
directory=$(go list -m -f '{{.Dir}}' "$module")
test "$directory" = "$(go env GOMODCACHE)/$module@v0.30.2"
cd "$directory"

# Validate ALL original files and edit counts before changing any file.
for file in acme/challenge.go authority/provisioner/aws.go authority/provisioner/azure.go authority/provisioner/oidc.go; do
    test -f "$file" && test ! -L "$file"
done
sha256sum -c <<'BEFORE'
527fbc84613fbb7221b738550b8c170bd9d0974c0070943b3370191a317129ee  acme/challenge.go
44555f3094cff9ad088f6f47bf70bdd2718146cb40382a5f6caef9fe154d8e69  authority/provisioner/aws.go
200f4f9fc40dc79dfacfb8e5411ebc7fb2b894c8d2bd8a300088b455b918bbeb  authority/provisioner/azure.go
830a70c021c1058f6a5bf00ec3a128b883cdf1bf36338f163670ae629844b5b2  authority/provisioner/oidc.go
BEFORE
test "$(grep -Fc 'Audience: jose.Audience{v.audience}' acme/challenge.go)" -eq 2
test "$(grep -Fc 'jose.Signed(signer).Claims(payload).CompactSerialize()' authority/provisioner/aws.go)" -eq 1
test "$(grep -Fc 'Audience: []string{p.Audience}' authority/provisioner/azure.go)" -eq 1
test "$(grep -Fc 'Audience: jose.Audience{o.ClientID}' authority/provisioner/oidc.go)" -eq 1

chmod u+w acme/challenge.go authority/provisioner/aws.go authority/provisioner/azure.go authority/provisioner/oidc.go
sed -i 's/Audience: jose\.Audience{v\.audience}/AnyAudience: jose.Audience{v.audience}/g' acme/challenge.go
sed -i 's/jose\.Signed(signer)\.Claims(payload)\.CompactSerialize()/jose.Signed(signer).Claims(payload).Serialize()/g' authority/provisioner/aws.go
sed -i 's/Audience: \[\]string{p\.Audience}/AnyAudience: []string{p.Audience}/g' authority/provisioner/azure.go
sed -i 's/Audience: jose\.Audience{o\.ClientID}/AnyAudience: jose.Audience{o.ClientID}/g' authority/provisioner/oidc.go
sha256sum -c <<'AFTER'
017767dc8d96285d96a5c686b667a4c100498b962f58392ba08b601df6c4c8c8  acme/challenge.go
d7319540b0869c622fc47323ab1f0d2d56d4f1c8fa29dc76ba81b17c9ea6391e  authority/provisioner/aws.go
3cc84dc259aa3ba220e1e9042a3c02d60145d401c5b1832607376879451a6045  authority/provisioner/azure.go
a6506666a302d52baf2bd8c16e7660f0a604d20ff5cb50924e3db8b2c27b7cc9  authority/provisioner/oidc.go
AFTER
chmod a-w acme/challenge.go authority/provisioner/aws.go authority/provisioner/azure.go authority/provisioner/oidc.go
