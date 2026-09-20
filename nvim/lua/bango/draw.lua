local M = {}

local layout = require "bango.layout"
local ns = vim.api.nvim_create_namespace "bango.panel"

M.groups = {
  title = "BangoTitle", subtitle = "BangoSubtitle", section = "BangoSection",
  note = "BangoNote", count = "BangoCount", ref = "BangoRef",
  fact = "BangoFact", footer = "BangoFooter", dim = "BangoDim",
  here = "BangoHere", waiting = "BangoWaiting", done = "BangoDone",
  dirty = "BangoDirty", blocked = "BangoBlocked", detached = "BangoDetached",
  working = "BangoWorking", new = "BangoNew",
  changed = "BangoChanged", comment = "BangoComment", string = "BangoString",
  number = "BangoNumber", keyword = "BangoKeyword", name = "BangoName",
  type = "BangoType",
}

function M.highlights()
  local link = function(from, to)
    vim.api.nvim_set_hl(0, from, { link = to, default = true })
  end
  link("BangoTitle", "Title")
  link("BangoSubtitle", "Comment")
  link("BangoSection", "Comment")
  link("BangoNote", "Comment")
  link("BangoCount", "WarningMsg")
  link("BangoRef", "Function")
  link("BangoFact", "Comment")
  link("BangoFooter", "Comment")
  link("BangoDim", "Comment")
  link("BangoHere", "Special")
  link("BangoWaiting", "WarningMsg")
  link("BangoDone", "DiagnosticOk")
  link("BangoDirty", "WarningMsg")
  link("BangoBlocked", "DiagnosticError")
  link("BangoDetached", "DiagnosticWarn")
  link("BangoWorking", "Special")
  link("BangoNew", "Comment")
  -- A tone links to what the colourscheme already calls that thing, so a panel
  -- of code looks like the editor it is being read in.
  link("BangoChanged", "DiffText")
  link("BangoComment", "Comment")
  link("BangoString", "String")
  link("BangoNumber", "Number")
  link("BangoKeyword", "Keyword")
  link("BangoName", "Identifier")
  link("BangoType", "Type")
end

local function style(col, row, here)
  if row.dim then
    return M.groups.dim
  end
  if col.kind == "count" then
    return M.groups.count
  end
  if col.kind == "ref" then
    return M.groups.ref
  end
  if col.kind == "time" then
    return M.groups.note
  end
  if here then
    return nil
  end
  return nil
end

function M.render(panel, state)
  local shown = layout.filter(panel, state.query)
  local lines = layout.rows(shown, state.folded)

  -- Two counts, and they are not the same one: whether there is anything to
  -- show at all, and which rows a column may be measured from.
  local shown, rows = 0, {}
  for _, line in ipairs(lines) do
    if not line.header then
      shown = shown + 1
      if not line.verbatim then
        table.insert(rows, line.row)
      end
    end
  end

  local text, marks, places = {}, {}, {}
  local add = function(body, group)
    table.insert(text, body)
    if group then
      table.insert(marks, { #text - 1, 0, -1, group })
    end
    table.insert(places, false)
  end

  add(panel.title or "", M.groups.title)
  if panel.subtitle and panel.subtitle ~= "" then
    add(panel.subtitle, M.groups.subtitle)
  end
  add("", nil)

  if shown == 0 then
    add(panel.empty or "nothing here", M.groups.dim)
    return { lines = text, marks = marks, places = places, rows = rows }
  end

  local cols = layout.columns(rows, state.width)
  for _, line in ipairs(lines) do
    if line.header then
      add(line.label, M.groups.section)
    else
      local row = line.row
      local at = #text
      local pieces = {}
      local cursor = row.id == state.cursor

      table.insert(pieces, cursor and layout.glyph("here", state.ascii) or " ")
      local mark = layout.glyph(row.mark, state.ascii)
      table.insert(pieces, mark)
      table.insert(pieces, " ")

      local spans = {}
      local column = layout.width(table.concat(pieces))

      -- A verbatim row is one value, written out: no column is measured
      -- against it, nothing pads it, and it carries no highlight, because
      -- what the value means is the producer's business, not a renderer's.
      if line.verbatim then
        local field = (row.fields or {})[1] or {}
        local value = field.value or ""
        -- A span is counted in runes and an extmark in bytes, so each edge is
        -- converted through the string itself rather than assumed equal.
        for _, span in ipairs(field.spans or {}) do
          local from = vim.str_byteindex(value, span.from, true)
          local to = vim.str_byteindex(value, math.min(span.to, vim.fn.strchars(value)), true)
          local group = M.groups[span.tone or ""]
          if group then
            table.insert(spans, { column + from, column + to, group })
          end
        end
        table.insert(pieces, value)
      end

      local values = {}
      for _, field in ipairs(row.fields or {}) do
        values[field.name] = field
      end
      for i, col in ipairs(line.verbatim and {} or cols) do
        local field = values[col.name] or {}
        local value = field.value or ""
        if i == 1 and line.depth > 0 then
          value = ("  "):rep(line.depth) .. value
        end
        local piece = layout.fit(value, col.kind, col.width)
        table.insert(spans, { column, column + #piece, style(col, row, cursor) })
        column = column + #piece
        table.insert(pieces, piece)
        if i < #cols then
          table.insert(pieces, " ")
          column = column + 1
        end
      end
      local body = table.concat(pieces)
      if row.note and row.note ~= "" then
        local note = " " .. row.note
        table.insert(spans, { #body, #body + #note,
          row.note:match "^%d" and M.groups.count or M.groups.note })
        body = body .. note
      end
      body = body:gsub("%s+$", "")

      table.insert(text, body)
      table.insert(places, row)
      if cursor then
        table.insert(marks, { at, 0, 1, M.groups.here })
      end
      if row.mark and row.mark ~= "" then
        table.insert(marks, { at, 1, 2, M.groups[row.mark] })
      end
      if row.dim then
        table.insert(marks, { at, 3, -1, M.groups.dim })
      else
        for _, span in ipairs(spans) do
          if span[3] then
            table.insert(marks, { at, span[1], span[2], span[3] })
          end
        end
      end

      if cursor then
        for _, fact in ipairs(row.facts or {}) do
          add("    " .. fact, M.groups.fact)
        end
        for _, preview in ipairs(row.preview or {}) do
          add("    " .. preview, M.groups.fact)
        end
      end
    end
  end

  if panel.hints and #panel.hints > 0 then
    add("", nil)
    add(table.concat(panel.hints, " · "), M.groups.footer)
  end
  return { lines = text, marks = marks, places = places, rows = rows }
end

function M.paint(buf, drawn)
  vim.bo[buf].modifiable = true
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, drawn.lines)
  vim.bo[buf].modifiable = false
  vim.api.nvim_buf_clear_namespace(buf, ns, 0, -1)
  for _, mark in ipairs(drawn.marks) do
    local line, from, to, group = mark[1], mark[2], mark[3], mark[4]
    if to == -1 then
      to = #(drawn.lines[line + 1] or "")
    end
    pcall(vim.api.nvim_buf_set_extmark, buf, ns, line, from, {
      end_col = math.min(to, #(drawn.lines[line + 1] or "")), hl_group = group,
    })
  end
end

return M
