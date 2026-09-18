local M = {}

M.gutter = 3
M.gap = 1
M.note_cap = 20
M.floors = { text = 6, path = 12, ref = 8 }

M.glyphs = {
  unicode = { here = "▸", new = "○", working = "◐", waiting = "⏎",
    done = "✓", dirty = "●", blocked = "⨯", detached = "⚠" },
  ascii = { here = ">", new = "o", working = "%", waiting = "!",
    done = "x", dirty = "*", blocked = "X", detached = "~" },
}

function M.glyph(mark, ascii)
  local set = ascii and M.glyphs.ascii or M.glyphs.unicode
  return set[mark or ""] or " "
end

function M.width(s)
  return vim.fn.strdisplaywidth(s or "")
end

local function floor(kind)
  return M.floors[kind == "" and "text" or kind] or M.floors.text
end

local function elastic(kind)
  return kind == nil or kind == "" or kind == "text"
end

function M.columns(rows, width)
  local cols, index = {}, {}
  for _, row in ipairs(rows) do
    for _, field in ipairs(row.fields or {}) do
      local at = index[field.name]
      if not at then
        table.insert(cols, { name = field.name, kind = field.kind or "", width = M.width(field.value) })
        index[field.name] = #cols
      else
        cols[at].width = math.max(cols[at].width, M.width(field.value))
        if cols[at].kind == "" and field.kind then
          cols[at].kind = field.kind
        end
      end
    end
  end
  if #cols == 0 then
    return cols
  end

  local note = 0
  for _, row in ipairs(rows) do
    note = math.max(note, M.width(row.note))
  end
  note = math.min(note, M.note_cap)

  local budget = math.max(width - M.gutter - note - M.gap * #cols, 1)
  local total = 0
  for _, col in ipairs(cols) do
    total = total + col.width
  end

  if total <= budget then
    local widest, at = -1, nil
    for i, col in ipairs(cols) do
      if elastic(col.kind) and col.width > widest then
        widest, at = col.width, i
      end
    end
    if at then
      cols[at].width = cols[at].width + (budget - total)
    end
    return cols
  end

  local need = total - budget
  for _, group in ipairs { { "text", "path" }, { "ref" } } do
    while need > 0 do
      local widest, at = -1, nil
      for i, col in ipairs(cols) do
        local kind = col.kind == "" and "text" or col.kind
        if vim.tbl_contains(group, kind) and col.width > floor(kind) and col.width > widest then
          widest, at = col.width, i
        end
      end
      if not at then
        break
      end
      cols[at].width = cols[at].width - 1
      need = need - 1
    end
  end
  return cols
end

local function take(value, width)
  if width <= 0 then
    return ""
  end
  local out, chars = "", vim.fn.strchars(value)
  for i = 0, chars - 1 do
    local ch = vim.fn.strcharpart(value, i, 1)
    if M.width(out .. ch) > width then
      break
    end
    out = out .. ch
  end
  return out
end

local function tail(value, width)
  if M.width(value) <= width then
    return value
  end
  if width < 2 then
    return take(value, width)
  end
  return take(value, width - 1) .. "…"
end

local function middle(value, width)
  local cut = value:match "^.*()/"
  if not cut then
    return tail(value, width)
  end
  local base = value:sub(cut + 1)
  local head = width - M.width(base) - 2
  if head < 1 then
    return tail(base, width)
  end
  return take(value, head) .. "…/" .. base
end

function M.fit(value, kind, width)
  value = value or ""
  if width <= 0 then
    return ""
  end
  local have = M.width(value)
  if have <= width then
    if kind == "count" then
      return (" "):rep(width - have) .. value
    end
    return value .. (" "):rep(width - have)
  end
  if kind == "path" then
    return middle(value, width)
  end
  if kind == "count" or kind == "time" then
    return value
  end
  return tail(value, width)
end

function M.rows(panel, folded)
  local out = {}
  local order = {}
  for _, section in ipairs(panel.sections or {}) do
    table.insert(order, section)
  end
  if panel.order then
    table.sort(order, function(a, b)
      local rank = function(id)
        for i, name in ipairs(panel.order) do
          if name == id then
            return i
          end
        end
        return #panel.order + 1
      end
      return rank(a.id) < rank(b.id)
    end)
  end
  for _, section in ipairs(order) do
    if section.label and section.label ~= "" then
      table.insert(out, { header = true, label = section.label, section = section.id })
    end
    if not section.collapsed then
      local function walk(row, depth)
        table.insert(out, { row = row, depth = depth })
        if not (folded or {})[row.id] then
          for _, child in ipairs(row.children or {}) do
            walk(child, depth + 1)
          end
        end
      end
      for _, row in ipairs(section.rows or {}) do
        walk(row, 0)
      end
    end
  end
  return out
end

function M.matches(row, query)
  if not query or vim.trim(query) == "" then
    return true
  end
  local hay = (row.id or "") .. " " .. (row.note or "")
  for _, field in ipairs(row.fields or {}) do
    hay = hay .. " " .. (field.value or "")
  end
  hay = hay:lower()
  for term in query:lower():gmatch "%S+" do
    if not hay:find(term, 1, true) then
      return false
    end
  end
  return true
end

function M.filter(panel, query)
  if not query or vim.trim(query) == "" then
    return panel
  end
  local out = vim.deepcopy(panel)
  local sections = {}
  for _, section in ipairs(out.sections or {}) do
    local rows = {}
    for _, row in ipairs(section.rows or {}) do
      local kept = {}
      for _, child in ipairs(row.children or {}) do
        if M.matches(child, query) then
          table.insert(kept, child)
        end
      end
      if #kept > 0 then
        row.children = kept
        table.insert(rows, row)
      elseif M.matches(row, query) then
        row.children = nil
        table.insert(rows, row)
      end
    end
    if #rows > 0 then
      section.rows = rows
      table.insert(sections, section)
    end
  end
  out.sections = sections
  return out
end

return M
