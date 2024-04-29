# ssh-agentx

<!-- TOC -->

- [ssh-agentx](#ssh-agentx)
  - [Rationale](#rationale)
  - [Requirements gpg signing](#requirements-gpg-signing)
  - [Requirements yubikey signing](#requirements-yubikey-signing)
  - [Configuration ssh-agentx gpg](#configuration-ssh-agentx-gpg)
  - [Configuration ssh-agentx yubikey](#configuration-ssh-agentx-yubikey)
  - [Configuration ssh-gpg-signer](#configuration-ssh-gpg-signer)
    - [Linux](#linux)
    - [Windows](#windows)
  - [Configuration relic yubikey](#configuration-relic-yubikey)
  - [Signing commits after configuration](#signing-commits-after-configuration)

<!-- /TOC -->

The x stands for eXtended or Xtra.

ssh-agentx is a ssh-agent replacement that abuses the SSH Extension protocol to sign git commits using your existing ssh keys (that you import into this agent).

When running under windows it also supports WSL/Pageant/WSL2/Cygwin thanks to the great <https://github.com/buptczq/WinCryptSSHAgent> tool.

It also now supports yubikey signing combined with relic on my fork - <https://github.com/42wim/relic/tree/sshtoken>

## Rationale

Because the one thing I need PGP for is to sign git commits AND I'm working mostly on (shared) remote servers.  
I don't want to setup a pgp/gpg configuration, keep a private key on the shared server and maintain it.  
As there is already remotely running a ssh-agent containing ed25519/rsa keys that can be used to do the same thing over the `SSH_AUTH_SOCK` socket.

The rationale above is gone now we can use ssh keys to sign git commits.

But this agent is being reused for code signing with yubikey over ssh, as code signing certificates now requires hardware tokens.  
In this setup you have a laptop with the yubikey and a remote server containing your builds.  

This setup is tested with a yubikey 5C

## Requirements gpg signing

If you only want to sign commits and never need to do `git log --show-signature` or `git verify-commit` you don't need gpg on the server.

You do need my companion tool that git will talk to when signing commits. See <https://github.com/42wim/ssh-gpg-signer>

## Requirements yubikey signing 

This tool works together with my fork of relic on <https://github.com/42wim/relic/tree/sshtoken>  
Go build this tool and see the section about relic configuration below.

You'll need to build this and make a relic.yml configuration file, you can find an example below:  

```yaml
---
tokens:
  ssh9a:
    type: ssh
    slot: "9a"
keys:
  ssh9a:
    token: ssh9a
    x509certificate: yourcertificate.crt
    slot: "9a"
```

After this you can use relic to sign an executable with

`relic sign -k ssh9a -f yourfile.exe -o yourfile-signed.exe`

So the setup is that on your laptop you're running ssh-agentx, you ssh into the server and there you run the relic command that will sign your executable using SSH extensions to talk to ssh-agentx which will talk to your yubikey plugged into your laptop.

## Configuration ssh-agentx gpg

If you want to run this agent instead of ssh-agent without the gpg signing stuff, you don't need a configuration.

Otherwise create a file called `ssh-agentx.toml` you can put in the same directory as `ssh-agentx` when testing or put it in `~/.config/ssh-agentx/ssh-agentx.toml` or `%APPDATA%\ssh-agentx\ssh-agentx.toml` on windows.

This file must contain a `[gpg.something]` header in case you have different git identities (you can use the same key for different identities if you want)

The `name` and `email` must match the email of your git configuration and the `matchcomment` must match the comment of your sshkey. (you can change comments of your keys using `ssh-keygen -c -f ~/.ssh/yourkey`).

You can also find the comment of your keys when running `ssh-add -l`

(:warning: It's better to create a new key to use solely for the gpg signing, read up on <https://security.stackexchange.com/questions/1806/why-should-one-not-use-the-same-asymmetric-key-for-encryption-as-they-do-for-sig> for why, you can still use an existing one if you want though)

```toml
[gpg.github]
name="yourname" #this must match your .gitconfig name
email="youremail" #this must match your .gitconfig email
matchcomment="akeycomment" #this must match a ssh key comment
```

So save this config above, start `ssh-agentx` and set your `SSH_AUTH_SOCK` path correct.

When you now add your key(s) to the agent `ssh-add ~/.ssh/ed25519` and it matches the `matchcomment` as above it'll give you a PGP public key block as shown below.

```text
2021/04/24 17:49:43 adding public key for yourname <youremail>
-----BEGIN PGP PUBLIC KEY BLOCK-----

xjMEAAAAABYJKwYBBAHaRw8BAQdAdN2uijeJajk1p9tJ+zaGR4ZtmxrrijPzJ195
1NKx8DDNFHlvdXJuYW1lIDx5b3VyZW1haWw+wogEExYIADoFAgAAAAAJEBTLefcM
08E9FiEERSpAhAOO4sCnMMBpFMt59wzTwT0CGwMCHgECGQEDCwkHAhUIAiIBAABf
AgEAuoHPX5vGBG95czyjHBxlfa3WKBEZKO5Oq9QYzy6Hq94A/02qShQlAkQs2Plz
Iaub4hgLmJWE1jk62pdjGP/VsIwA
=KL1J
-----END PGP PUBLIC KEY BLOCK-----
```

You can now copy this in your github or gitea GPG settings.

This concludes the agent side configuration, you also need the companion which will interact with git to sign it and send it to ssh-agentx.

## Configuration ssh-agentx yubikey

If you want to run this agent instead of ssh-agent without the yubikey signing stuff, you don't need a configuration.

Otherwise create a file called `ssh-agentx.toml` you can put in the same directory as `ssh-agentx` when testing or put it in `~/.config/ssh-agentx/ssh-agentx.toml` or `%APPDATA%\ssh-agentx\ssh-agentx.toml` on windows.

The `enable=true` is needed to actually use the yubikey signing part of ssh-agentx

```toml
[yubikey]
enable=true #needed to enable yubikey signing
enablelog=true #enable logging about yubikey operations
defaultslot="9a" #define the default yubikey slot to use (9a is the default authentication one)
```

Below is an example of the logs when signing
```
2024/04/29 23:19:24 got ssh-yubi-setslot@42wim setting slot to 9a
2024/04/29 23:19:24 got ssh-yubi-setslot@42wim new crypto signers set
2024/04/29 23:19:24 got ssh-yubi-publickey@42wim request for publickey
2024/04/29 23:19:24 got ssh-yubi-setslot@42wim setting slot to 9a but already set.
2024/04/29 23:19:24 got ssh-yubi-publickey@42wim request for publickey
2024/04/29 23:19:24 got ssh-yubi-setslot@42wim setting slot to 9a but already set.
2024/04/29 23:19:24 got ssh-yubi-sign@42wim request to sign
```

## Configuration ssh-gpg-signer

### Linux

Download/build <https://github.com/42wim/ssh-gpg-signer> and put the binary somewhere, lets assume `/home/user/bin/ssh-gpg-signer`.

Now change your global or local gitconfig to use ssh-gpg-signer and always sign git commits

```bash
git config --global gpg.program /home/user/bin/ssh-gpg-signer
git config --global commit.gpgSign true
```

### Windows

Download/build <https://github.com/42wim/ssh-gpg-signer> and put the binary somewhere, lets assume `c:\users\user\bin\ssh-gpg-signer`.

Now change your global or local gitconfig to use ssh-gpg-signer and always sign git commits

```bash
git config --global gpg.program c:\\users\\user\\bin\\ssh-gpg-signer
git config --global commit.gpgSign true
```

## Configuration relic yubikey

This tool works together with my fork of relic on <https://github.com/42wim/relic/tree/sshtoken>  
You'll need to build this and make a relic.yml configuration file, you can find an example below:  

```yaml
---
tokens:
  ssh9a:
    type: ssh
    slot: "9a"
keys:
  ssh9a:
    token: ssh9a
    x509certificate: yourcertificate.crt
    slot: "9a"
```

After this you can use relic to sign an executable with

`relic sign -k ssh9a -f yourfile.exe -o yourfile-signed.exe`

Running relic for the first time will get you a PIN code popup to access your yubikey for signing.   
Warning: Follow-up signing requests will use the cached pin (and won't need any interaction), so if you didn't specify a touch policy for your yubikey be sure to exit your SSH session, stop/kill ssh-agentx or just remove your yubikey when done signing.

For clarification: the setup is that on your laptop you're running ssh-agentx, you ssh into the server and there you run the relic command that will sign your executable using SSH extensions to talk to ssh-agentx which will talk to your yubikey plugged into your laptop.

## Signing commits after configuration

Now git will automatically sign your commits using `ssh-gpg-signer` which talks over the `SSH_AUTH_SOCK` socket to the `ssh-agentx`.

So just run `git commit -m "acommit"`

If you have `gpg` installed and you run `git log --show-signature` it'll show you something like this:

```git
commit 73e3d4e2a897c921f207f5a1ae65c7b6175b1afe (HEAD -> master)
gpg: Signature made Sat 24 Apr 2021 05:18:00 PM CEST
gpg:                using EDDSA key 452A4084038EE2C0A730C06914CB79F70CD3C13D
gpg: Good signature from "yourname <youremail>" [uncertain]
gpg: WARNING: This key is not certified with a trusted signature!
gpg:          There is no indication that the signature belongs to the owner.
Primary key fingerprint: 452A 4084 038E E2C0 A730  C069 14CB 79F7 0CD3 C13D
Author:     yourname <youremail>
AuthorDate: Fri Apr 23 22:26:45 2021 +0200
Commit:     yourname <youremail>
CommitDate: Fri Apr 23 22:26:45 2021 +0200

    acommit
```
