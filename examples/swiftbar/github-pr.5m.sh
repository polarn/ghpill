#!/bin/bash
# <swiftbar.title>GitHub</swiftbar.title>
# <swiftbar.desc>Pull requests, review requests and workflow runs, with a dropdown.</swiftbar.desc>
# <swiftbar.hideAbout>true</swiftbar.hideAbout>
# <swiftbar.hideRunInTerminal>true</swiftbar.hideRunInTerminal>
# <swiftbar.hideDisablePlugin>true</swiftbar.hideDisablePlugin>

export PATH="/opt/homebrew/bin:/usr/local/bin:$PATH"
exec ghpill --swiftbar "$@"
