package.path = "nvim/lua/?.lua;nvim/lua/?/init.lua;" .. package.path
local layout = require "bango.layout"
local draw = require "bango.draw"
local failures = 0
local function check(name, ok, detail)
  if not ok then
    failures = failures + 1
    print("FAIL " .. name .. (detail and ("\n  " .. tostring(detail)) or ""))
  end
end

local panel = vim.json.decode(table.concat(vim.fn.readfile "fixtures/confirm.json", "\n"))

local cols = layout.columns({ panel.sections[1].rows[1] }, 72)
check("columns come from the fields", #cols == 4, #cols)

check("a path keeps its basename",
  layout.fit("web-client/lib/utils/index.ts", "path", 20):match "index%.ts$" ~= nil,
  layout.fit("web-client/lib/utils/index.ts", "path", 20))
check("a path fits exactly", layout.width(layout.fit("web-client/lib/utils/index.ts", "path", 20)) == 20)
check("a count is right aligned", layout.fit("7", "count", 4) == "   7", layout.fit("7", "count", 4))
check("a time is never cut", layout.fit("18m", "time", 2) == "18m")

local rows = layout.rows(panel, {})
local headers, bodies = 0, 0
for _, line in ipairs(rows) do
  if line.header then headers = headers + 1 else bodies = bodies + 1 end
end
check("sections become headers", headers == 2, headers)
check("every row is drawn", bodies == 3, bodies)

local filtered = layout.filter(panel, "swallowed")
check("filtering keeps only what matches",
  #filtered.sections == 1 and #filtered.sections[1].rows == 1, #filtered.sections)

local drawn = draw.render(panel, { query = "", folded = {}, cursor = "t7", width = 72 })
check("the cursor row shows its facts",
  table.concat(drawn.lines, "\n"):match "claimed" ~= nil)
check("marks are placed", #drawn.marks > 0, #drawn.marks)
local widest = 0
for _, line in ipairs(drawn.lines) do widest = math.max(widest, layout.width(line)) end
check("nothing overflows the width", widest <= 72, widest)

if failures > 0 then os.exit(1) end
print("bango.nvim native renderer ok")
