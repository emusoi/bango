local M = {}

local layout = require "bango.layout"
local draw = require "bango.draw"

local spellings = {
  ["⏎"] = { "<CR>", "enter" },
  ["⇥"] = { "<Tab>" },
  ["⎋"] = { "<Esc>" },
}

local reserved = { ["/"] = true, ["?"] = true, ["q"] = true, ["<Esc>"] = true }

function M.open(opts)
  local panel = opts.panel
  if not panel or panel.bango ~= 1 then
    vim.notify("bango: that is not a panel", vim.log.levels.ERROR)
    return
  end
  draw.highlights()

  local state = {
    query = "", folded = {}, cursor = nil, ascii = opts.ascii,
    width = 0, at = 1,
  }

  local columns = math.floor(vim.o.columns * (opts.width or 0.7))
  local rows = math.floor(vim.o.lines * (opts.height or 0.6))
  local previous = vim.api.nvim_get_current_win()
  local buf = vim.api.nvim_create_buf(false, true)
  local win = vim.api.nvim_open_win(buf, true, {
    relative = "editor", width = columns, height = rows,
    row = math.floor((vim.o.lines - rows) / 2) - 1,
    col = math.floor((vim.o.columns - columns) / 2),
    style = "minimal", border = "rounded",
    title = opts.title and (" " .. opts.title .. " ") or nil,
  })
  state.width = columns - 2
  vim.bo[buf].filetype = "bango"
  vim.bo[buf].bufhidden = "wipe"
  vim.bo[buf].modifiable = false
  vim.wo[win].cursorline = true
  vim.wo[win].wrap = false

  local drawn
  local closed = false

  local function close()
    if closed then
      return
    end
    closed = true
    pcall(vim.api.nvim_win_close, win, true)
    if vim.api.nvim_win_is_valid(previous) then
      pcall(vim.api.nvim_set_current_win, previous)
    end
  end

  local function current()
    if not drawn then
      return nil
    end
    return drawn.places[vim.api.nvim_win_get_cursor(win)[1]] or nil
  end

  local function repaint(keep)
    drawn = draw.render(panel, state)
    draw.paint(buf, drawn)
    local line = keep
    if not line then
      for n, place in ipairs(drawn.places) do
        if place then
          line = n
          break
        end
      end
    end
    if line then
      pcall(vim.api.nvim_win_set_cursor, win, { math.min(line, #drawn.lines), 0 })
    end
  end

  local function settle()
    local row = current()
    if not row then
      return
    end
    if state.cursor ~= row.id then
      state.cursor = row.id
      local line = vim.api.nvim_win_get_cursor(win)[1]
      repaint(line)
      if opts.on_move then
        opts.on_move(row, function()
          if vim.api.nvim_win_is_valid(win) then
            vim.api.nvim_set_current_win(win)
          end
        end)
      end
    end
  end

  local function move(step)
    if not drawn then
      return
    end
    local line = vim.api.nvim_win_get_cursor(win)[1]
    local n = line
    repeat
      n = n + step
      if n < 1 or n > #drawn.lines then
        return
      end
    until drawn.places[n]
    pcall(vim.api.nvim_win_set_cursor, win, { n, 0 })
    settle()
  end

  local function choose(name, row, input, chosen)
    close()
    if opts.on_choice then
      opts.on_choice { action = name, row = row.target or row.id,
        input = input, choice = chosen }
    end
  end

  local function act(name)
    local row = current()
    if not row then
      return
    end
    local action = (panel.actions or {})[name]
    if not action then
      return
    end
    if action.confirm and vim.fn.confirm(action.confirm, "&yes\n&no", 2) ~= 1 then
      return
    end
    if action.choices and #action.choices > 0 then
      return vim.ui.select(action.choices, { prompt = action.label or name }, function(pick)
        if pick then
          choose(name, row, nil, pick)
        end
      end)
    end
    if action.input and action.input ~= "" then
      return vim.ui.input({ prompt = action.input .. " " }, function(value)
        if value and value ~= "" then
          choose(name, row, value)
        end
      end)
    end
    choose(name, row)
  end

  local function allows(row, name)
    if (panel.actions[name] or {}).global then
      return true
    end
    for _, allowed in ipairs(row.actions or {}) do
      if allowed == name then
        return true
      end
    end
    return false
  end

  local map = function(key, action)
    vim.keymap.set("n", key, action, { buffer = buf, nowait = true })
  end

  map("q", close)
  map("<Esc>", close)
  map("j", function() move(1) end)
  map("k", function() move(-1) end)
  map("<Down>", function() move(1) end)
  map("<Up>", function() move(-1) end)
  map("G", function()
    for n = #drawn.lines, 1, -1 do
      if drawn.places[n] then
        pcall(vim.api.nvim_win_set_cursor, win, { n, 0 })
        return settle()
      end
    end
  end)
  map("g", function() repaint(); settle() end)
  map("<Space>", function()
    local row = current()
    if row and row.children and #row.children > 0 then
      state.folded[row.id] = not state.folded[row.id]
      repaint(vim.api.nvim_win_get_cursor(win)[1])
    end
  end)
  map("/", function()
    vim.ui.input({ prompt = "/" }, function(value)
      state.query = value or ""
      repaint()
      settle()
    end)
  end)
  map("?", function()
    local help = { "actions", "" }
    local names = vim.tbl_keys(panel.actions or {})
    table.sort(names)
    for _, name in ipairs(names) do
      local action = panel.actions[name]
      table.insert(help, ("  %-4s %s%s"):format(action.key or "",
        action.label or name, action.help and ("  — " .. action.help) or ""))
    end
    vim.list_extend(help, { "", "  /    filter", "  q    close" })
    vim.notify(table.concat(help, "\n"))
  end)

  local bound = {}
  local names = vim.tbl_keys(panel.actions or {})
  table.sort(names)
  for _, name in ipairs(names) do
    local action = panel.actions[name]
    local keys = { action.key }
    for _, spelling in ipairs(spellings[action.key] or {}) do
      table.insert(keys, spelling)
    end
    for _, key in ipairs(keys) do
      if key and key ~= "" and not reserved[key] then
        bound[key] = bound[key] or {}
        table.insert(bound[key], name)
      end
    end
  end

  for key, candidates in pairs(bound) do
    map(key, function()
      local row = current()
      if not row then
        return
      end
      for _, name in ipairs(candidates) do
        if allows(row, name) then
          return act(name)
        end
      end
    end)
  end

  vim.api.nvim_create_autocmd("CursorMoved", {
    buffer = buf,
    callback = function() settle() end,
  })

  repaint()
  settle()
  return { win = win, buf = buf, close = close }
end

return M
