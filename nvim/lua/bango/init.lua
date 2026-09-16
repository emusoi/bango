local M = {}

M.bin = "bango"

local function escape(argv)
  local out = {}
  for i, arg in ipairs(argv) do
    out[i] = vim.fn.shellescape(arg)
  end
  return table.concat(out, " ")
end

function M.command(opts, out)
  local bango = { M.bin, "--json" }
  if opts.ascii then
    table.insert(bango, "--ascii")
  end
  for _, flag in ipairs(opts.args or {}) do
    table.insert(bango, flag)
  end
  if opts.drive then
    local argv = vim.list_extend(vim.deepcopy(bango), { "--" })
    return escape(vim.list_extend(argv, opts.cmd)) .. " > " .. vim.fn.shellescape(out)
  end
  return escape(opts.cmd) .. " | " .. escape(bango) .. " > " .. vim.fn.shellescape(out)
end

local function float(opts)
  local columns = math.floor(vim.o.columns * (opts.width or 0.8))
  local lines = math.floor(vim.o.lines * (opts.height or 0.7))
  local buf = vim.api.nvim_create_buf(false, true)
  local win = vim.api.nvim_open_win(buf, true, {
    relative = "editor",
    width = columns,
    height = lines,
    row = math.floor((vim.o.lines - lines) / 2) - 1,
    col = math.floor((vim.o.columns - columns) / 2),
    style = "minimal",
    border = "rounded",
  })
  return buf, win
end

function M.panel(opts)
  return require("bango.native").open(opts)
end

function M.read(cmd, cwd)
  local done = vim.system(cmd, { text = true, cwd = cwd }):wait()
  if done.code ~= 0 then
    return nil, vim.trim(done.stderr or "the producer failed")
  end
  local ok, panel = pcall(vim.json.decode, done.stdout,
    { luanil = { object = true, array = true } })
  if not ok then
    return nil, "the producer did not print a panel"
  end
  return panel
end

function M.pick(opts)
  if vim.fn.executable(M.bin) == 0 then
    vim.notify("bango: not on PATH", vim.log.levels.ERROR)
    return
  end
  local out = vim.fn.tempname()
  local complaint = vim.fn.tempname()
  local previous = vim.api.nvim_get_current_win()
  local buf, win = float(opts)

  vim.fn.termopen({ "sh", "-c", M.command(opts, out) .. " 2> " .. vim.fn.shellescape(complaint) }, {
    on_exit = function(_, code)
      if vim.api.nvim_win_is_valid(win) then
        vim.api.nvim_win_close(win, true)
      end
      if vim.api.nvim_win_is_valid(previous) then
        vim.api.nvim_set_current_win(previous)
      end
      local said = ""
      if vim.fn.filereadable(complaint) == 1 then
        said = vim.trim(table.concat(vim.fn.readfile(complaint), "\n"))
      end
      vim.fn.delete(complaint)
      if code ~= 0 and code ~= 130 then
        vim.fn.delete(out)
        vim.notify(said ~= "" and said or ("bango exited " .. code), vim.log.levels.ERROR)
        return
      end
      if said ~= "" then
        vim.notify(said, vim.log.levels.WARN)
      end
      if vim.fn.filereadable(out) == 0 then
        vim.fn.delete(out)
        return
      end
      local body = table.concat(vim.fn.readfile(out), "\n")
      vim.fn.delete(out)
      if body == "" then
        return
      end
      local ok, choice = pcall(vim.json.decode, body, { luanil = { object = true, array = true } })
      if ok and opts.on_choice then
        opts.on_choice(choice)
      end
    end,
  })
  vim.api.nvim_buf_set_option(buf, "bufhidden", "wipe")
  vim.cmd.startinsert()
end

return M
