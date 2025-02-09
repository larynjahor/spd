# spd – Simple Package Driver
## What
`spd` implements custom `GOPACKAGESDRIVER` for gopls.
Unlike `go list`, `spd` scans only given directories via `SPDTARGETS` variable allowing to run gopls in large repos.
## Editor support
### VSCode
Be aware to run VSCode from the root directory.
```json
{
  "gopls": {
    "build.subdirWatchPatterns": "off",
    "build.importsSource": "off", // may be unnecessary in some cases
    "build.directoryFilters": [
        "-",
        "+prj1",
        "+prj2",
        "+prj3",
    ],
    "build.env": {
        "GOPACKAGESDRIVER": "path to spd",
        "SPDTARGETS": "prj1,prj2,prj3",
        "GOFLAGS": "-mod=vendor", // may apply
    }
  }
}
```
### Neovim
```lua
local lspconfig = require("lspconfig")

local targets = {
    "prj1",
    "prj2",
    "prj3",
}

local gopls_config = {
    settings = {
        gopls = {
            subdirWatchPatterns = "off",
            importsSource = "off", -- may be unnecessary in some cases
            env = {
                "GOPACKAGESDRIVER" = "path to spd",
                "SPDTARGETS" = table.concat(targets, ","),
                "GOFLAGS" = "-mod=vendor", -- may apply
            },
        },
    }
}

gopls_config.settings.gopls.directoryFilters = {
    "-"
}

for _, target in ipairs(targets) do
    table.insert(gopls_config.settings.gopls.directoryFilters, "+"..target)
end

lspconfig.gopls.setup(gopls_config)
```
