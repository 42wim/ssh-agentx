# ssh-agentx

<!-- TOC -->

- [ssh-agentx](#ssh-agentx)
  - [Rationale](#rationale)
  - [Requirements](#requirements)
  - [Configuration ssh-agentx](#configuration-ssh-agentx)
  - [Configuration ssh-gpg-signer](#configuration-ssh-gpg-signer)
    - [Linux](#linux)
    - [Windows](#windows)
  - [Signing commits after configuration](#signing-commits-after-configuration)

<!-- /TOC -->

The x stands for eXtended or Xtra.

ssh-agentx is a ssh-agent replacement that abuses the SSH Extension protocol to sign git commits using your existing ssh keys (that you import into this agent).

When running under windows it also supports WSL/Pageant/WSL2/Cygwin thanks to the great <https://github.com/buptczq/WinCryptSSHAgent> tool.

## Rationale

Because the one thing I need PGP for is to sign git commits AND I'm working mostly on (shared) remote servers.  
I don't want to setup a pgp/gpg configuration, keep a private key on the shared server and maintain it.  
As there is already remotely running a ssh-agent containing ed25519/rsa keys that can be used to do the same thing over the `SSH_AUTH_SOCK` socket.

## Requirements

If you only want to sign commits and never need to do `git log --show-signature` or `git verify-commit` you don't need gpg on the server.

You do need my companion tool that git will talk to when signing commits. See <https://github.com/42wim/ssh-gpg-signer>

## Configuration ssh-agentx

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
