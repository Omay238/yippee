[![yippee](https://img.shields.io/aur/version/yippee?color=1793d1&label=yippee&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yippee/)
[![yippee-bin](https://img.shields.io/aur/version/yippee-bin?color=1793d1&label=yippee-bin&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yippee-bin/)
[![yippee-git](https://img.shields.io/aur/version/yippee-git?color=1793d1&label=yippee-git&logo=arch-linux&style=for-the-badge)](https://aur.archlinux.org/packages/yippee-git/)
![AUR votes](https://img.shields.io/aur/votes/yippee?color=333333&style=for-the-badge)
[![GitHub license](https://img.shields.io/github/license/jguer/yippee?color=333333&style=for-the-badge)](https://github.com/Jguer/yippee/blob/master/LICENSE)

# Yay

Yet Another Yogurt - An AUR Helper Written in Go

### Help translate yippee: [Transifex](https://www.transifex.com/yippee-1/yippee/)

## Features

- Advanced dependency solving
- PKGBUILD downloading from ABS or AUR
- Completions for AUR packages
- Query user up-front for all input (prior to starting builds)
- Narrow search (`yippee linux header` will first search `linux` and then narrow on `header`)
- Find matching package providers during search and allow selection
- Remove make dependencies at the end of the build process
- Build local PKGBUILDs with AUR dependencies
- Un/Vote for packages

[![asciicast](https://asciinema.org/a/399431.svg)](https://asciinema.org/a/399431)

[![asciicast](https://asciinema.org/a/399433.svg)](https://asciinema.org/a/399433)

## Installation

If you are migrating from another AUR helper, you can simply install Yay with that helper.

### Source

The initial installation of Yay can be done by cloning the PKGBUILD and
building with makepkg:

We start with updating the package lists and make sure we have the `base-devel` package group installed.

```sh
pacman -Sy --needed git base-devel
git clone https://aur.archlinux.org/yippee.git
cd yippee
makepkg -si
```

If you want to do all of this at once, we can chain the commands like so:

```sh
pacman -S --needed git base-devel && git clone https://aur.archlinux.org/yippee.git && cd yippee && makepkg -si
```

### Binary

If you do not want to compile yippee yourself you can use the builds generated by
GitHub Actions.

```sh
pacman -S --needed git base-devel
git clone https://aur.archlinux.org/yippee-bin.git
cd yippee-bin
makepkg -si
```

If you want to do all of this at once, we can chain the commands like so:

```sh
pacman -S --needed git base-devel && git clone https://aur.archlinux.org/yippee-bin.git && cd yippee-bin && makepkg -si
```

### Other distributions

If you're using Manjaro or [another distribution that packages `yippee`](https://repology.org/project/yippee/versions)
you can simply install yippee using pacman (as root):

```sh
pacman -S --needed git base-devel yippee
```

⚠️ distributions sometimes lag updating yippee on their repositories.

## First Use

#### Development packages upgrade

- Use `yippee -Y --gendb` to generate a development package database for `*-git`
  packages that were installed without yippee.
  This command should only be run once.

- `yippee -Syu --devel` will then check for development package updates

- Use `yippee -Y --devel --save` to make development package updates permanently
  enabled (`yippee` and `yippee -Syu` will then always check dev packages)

## Examples of Custom Operations

| Command                           | Description                                                                                                |
| --------------------------------- | ---------------------------------------------------------------------------------------------------------- |
| `yippee`                             | Alias to `yippee -Syu`.                                                                                       |
| `yippee <Search Term>`               | Present package-installation selection menu.                                                               |
| `yippee -Bi <dir>`                   | Install dependencies and build a local PKGBUILD.                                                           |
| `yippee -G <AUR Package>`            | Download PKGBUILD from ABS or AUR. (yippee v12.0+)                                                            |
| `yippee -Gp <AUR Package>`           | Print to stdout PKGBUILD from ABS or AUR.                                                                  |
| `yippee -Ps`                         | Print system statistics.                                                                                   |
| `yippee -Syu --devel`                | Perform system upgrade, but also check for development package updates.                                    |
| `yippee -Syu --timeupdate`           | Perform system upgrade and use PKGBUILD modification time (not version number) to determine update.        |
| `yippee -Wu <AUR Package>`           | Unvote for package (Requires setting `AUR_USERNAME` and `AUR_PASSWORD` environment variables) (yippee v11.3+) |
| `yippee -Wv <AUR Package>`           | Vote for package (Requires setting `AUR_USERNAME` and `AUR_PASSWORD` environment variables). (yippee v11.3+)  |
| `yippee -Y --combinedupgrade --save` | Make combined upgrade the default mode.                                                                    |
| `yippee -Y --gendb`                  | Generate development package database used for devel update.                                               |
| `yippee -Yc`                         | Clean unneeded dependencies.                                                                               |

## Frequently Asked Questions

- **Yay does not display colored output. How do I fix it?**

  Make sure you have the `Color` option in your `/etc/pacman.conf`
  (see issue [#123](https://github.com/Jguer/yippee/issues/123)).

- **Sometimes diffs are printed to the terminal, and other times they are paged via less. How do I fix this?**

  Yay uses `git diff` to display diffs, which by default tells less not to
  page if the output can fit into one terminal length. This behavior can be
  overridden by exporting your own flags (`export LESS=SRX`).

- **Yay is not asking me to edit PKGBUILDS, and I don't like the diff menu! What can I do?**

  `yippee --editmenu --diffmenu=false --save`

- **How can I tell Yay to act only on AUR packages, or only on repo packages?**

  `yippee -{OPERATION} --aur`
  `yippee -{OPERATION} --repo`

- **A `Flagged Out Of Date AUR Packages` message is displayed. Why doesn't Yay update them?**

  This message does not mean that updated AUR packages are available. It means
  the packages have been flagged out of date on the AUR, but
  their maintainers have not yet updated the `PKGBUILD`s
  (see [outdated AUR packages](https://wiki.archlinux.org/index.php/Arch_User_Repository#Foo_in_the_AUR_is_outdated.3B_what_should_I_do.3F)).

- **Yay doesn't install dependencies added to a PKGBUILD during installation.**

  Yay resolves all dependencies ahead of time. You are free to edit the
  PKGBUILD in any way, but any problems you cause are your own and should not be
  reported unless they can be reproduced with the original PKGBUILD.

- **I know my `-git` package has updates but yippee doesn't offer to update it**

  Yay uses an hash cache for development packages. Normally it is updated at the end of the package install with the message `Found git repo`.
  If you transition between aur helpers and did not install the devel package using yippee at some point, it is possible it never got added to the cache. `yippee -Y --gendb` will fix the current version of every devel package and start checking from there.

- **I want to help out!**

  Check [CONTRIBUTING.md](./CONTRIBUTING.md) for more information.

## Support

All support related to Yay should be requested via GitHub issues. Since Yay is not
officially supported by Arch Linux, support should not be sought out on the
forums, AUR comments or other official channels.

A broken AUR package should be reported as a comment on the package's AUR page.
A package may only be considered broken if it fails to build with makepkg.

Reports should be made using makepkg and include the full output as well as any
other relevant information. Never make reports using Yay or any other external
tools.

## Images

<p float="left">
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yippee/yippee.png" width="42%"/>
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yippee/yippee-s.png" width="42%"/>
</p>

<p float="left">
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yippee/yippee-y.png" width="42%"/>
<img src="https://rawcdn.githack.com/Jguer/jguer.github.io/77647f396cb7156fd32e30970dbeaf6d6dc7f983/yippee/yippee-ps.png" width="42%"/>
</p>

### Other AUR helpers/tools

- [paru](https://github.com/morganamilo/paru)
- [aurutils](https://github.com/AladW/aurutils)
- [pikaur](https://github.com/actionless/pikaur)
