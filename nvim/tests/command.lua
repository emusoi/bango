package.path = "nvim/lua/?.lua;nvim/lua/?/init.lua;" .. package.path
local bango = require "bango"
local failures = 0

local function check(name, got, want)
  if got ~= want then
    failures = failures + 1
    print("FAIL " .. name .. "\n  got  " .. got .. "\n  want " .. want)
  end
end

check("select mode pipes the producer in",
  bango.command({ cmd = { "loco", "panel", "confirm" } }, "/tmp/o"),
  "'loco' 'panel' 'confirm' | 'bango' '--json' > '/tmp/o'")

check("drive mode hands the producer over",
  bango.command({ cmd = { "mia", "api", "dashboard", "--bango" }, drive = true }, "/tmp/o"),
  "'bango' '--json' '--' 'mia' 'api' 'dashboard' '--bango' > '/tmp/o'")

check("a producer argument with a space stays one argument",
  bango.command({ cmd = { "loco", "threads", "--grep", "two words" } }, "/tmp/o"),
  "'loco' 'threads' '--grep' 'two words' | 'bango' '--json' > '/tmp/o'")

check("a quote cannot escape the command",
  bango.command({ cmd = { "loco", "-m", "it's; rm -rf /" } }, "/tmp/o"),
  [['loco' '-m' 'it'\''s; rm -rf /' | 'bango' '--json' > '/tmp/o']])

check("extra flags reach bango",
  bango.command({ cmd = { "x" }, ascii = true, args = { "--via-ssh", "box" } }, "/tmp/o"),
  "'x' | 'bango' '--json' '--ascii' '--via-ssh' 'box' > '/tmp/o'")

if failures > 0 then
  os.exit(1)
end
print("bango.nvim command construction ok")
