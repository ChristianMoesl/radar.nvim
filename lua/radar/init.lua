local M = {}

local config = {
	radar_cmd = "radar",
	auto_start = true,
	refresh_ms = 30000,
	width = 90,
	height = 24,
	notify_new_items = true,
	icons = {
		immediate = "🚨",
		attention = "👀",
		in_progress = "⏳",
		done = "✅",
	},
}

local state = {
	summary = { immediate = 0, attention = 0, in_progress = 0, done = 0 },
	items = {},
	timer = nil,
	buf = nil,
	win = nil,
	line_items = {},
	seen_items = {},
	seen_items_initialized = false,
}

local function decode_json(text)
	local ok, decoded = pcall(vim.json.decode, text)
	if ok then
		return decoded
	end
end

local function run(args, cb)
	vim.system(args, { text = true }, function(result)
		vim.schedule(function()
			cb(result)
		end)
	end)
end

local function start_daemon()
	if not config.auto_start then
		return
	end
	vim.fn.jobstart({ config.radar_cmd, "daemon" }, { detach = true })
end

local function restart_daemon()
	vim.fn.jobstart({ config.radar_cmd, "restart" }, {
		detach = true,
		on_exit = vim.schedule_wrap(function()
			M.load()
		end),
	})
end

local function stop_daemon()
	vim.fn.jobstart({ config.radar_cmd, "stop" }, { detach = true })
end

local function is_open()
	return state.win and vim.api.nvim_win_is_valid(state.win)
end

local function close_window()
	if is_open() then
		vim.api.nvim_win_close(state.win, true)
	end
	state.win = nil
end

local function open_url(url)
	if not url or url == "" then
		vim.notify("Radar item has no URL", vim.log.levels.WARN)
		return
	end

	if vim.ui and vim.ui.open then
		vim.ui.open(url)
	else
		vim.fn.jobstart({ "xdg-open", url }, { detach = true })
	end
end

local function item_icon(attention)
	return config.icons[attention] or "•"
end

local function item_label(attention)
	return ({
		immediate = "Needs immediate attention",
		attention = "Needs attention",
		in_progress = "In progress",
		done = "Done today",
	})[attention] or attention or "Unknown"
end

local function item_id(item)
	return item.id or string.format("%s:%s:%s", item.kind or "", item.repo or "", item.title or "")
end

local function notification_level(item)
	if item.attention == "immediate" then
		return vim.log.levels.WARN
	end
	return vim.log.levels.INFO
end

local function notify_new_items(items)
	local next_seen = {}
	local new_items = {}

	for _, item in ipairs(items) do
		local id = item_id(item)
		next_seen[id] = true
		if state.seen_items_initialized and not state.seen_items[id] then
			table.insert(new_items, item)
		end
	end

	state.seen_items = next_seen
	state.seen_items_initialized = true

	if not config.notify_new_items then
		return
	end

	for _, item in ipairs(new_items) do
		vim.notify(
			string.format("%s %s\n%s", item_icon(item.attention), item.title or "Untitled", item.reason or item_label(item.attention)),
			notification_level(item),
			{ title = "New Radar item" }
		)
	end
end

local function add_item(lines, line_items, item)
	table.insert(lines, string.format("%s %s", item_icon(item.attention), item.title or "Untitled"))
	line_items[#lines] = item
	table.insert(lines, string.format("   Status : %s", item_label(item.attention)))
	table.insert(lines, string.format("   Reason : %s", item.reason or "—"))
	table.insert(lines, string.format("   Kind   : %s", item.kind or "—"))
	if item.repo and item.repo ~= "" then
		table.insert(lines, string.format("   Repo   : %s", item.repo))
	end
	if item.url and item.url ~= "" then
		table.insert(lines, string.format("   URL    : %s", item.url))
	end
	if item.done_at and item.done_at ~= "" then
		table.insert(lines, string.format("   Done   : %s", item.done_at))
	end
	if item.id and item.id ~= "" then
		table.insert(lines, string.format("   ID     : %s", item.id))
	end
	if item.entities and #item.entities > 0 then
		table.insert(lines, "   Entities:")
		for _, entity in ipairs(item.entities) do
			local label = string.format("     - %s/%s", entity.source or "?", entity.kind or "?")
			if entity.status and entity.status ~= "" then
				label = label .. string.format(" [%s]", entity.status)
			end
			table.insert(lines, label)
			if entity.branch and entity.branch ~= "" then
				table.insert(lines, string.format("       branch: %s", entity.branch))
			end
			if entity.path and entity.path ~= "" then
				table.insert(lines, string.format("       path: %s", entity.path))
			end
			if entity.url and entity.url ~= "" then
				table.insert(lines, string.format("       url: %s", entity.url))
			end
		end
	end
	table.insert(lines, "")
end

local function render_lines()
	local s = state.summary
	local lines = {
		"Radar",
		string.format("%s %d immediate   %s %d attention   %s %d in progress   %s %d done", config.icons.immediate, s.immediate or 0, config.icons.attention, s.attention or 0, config.icons.in_progress, s.in_progress or 0, config.icons.done, s.done or 0),
		"",
		"<CR>: open URL    r: refresh    q/<Esc>: close",
		"",
	}
	local line_items = {}
	local groups = {
		{ key = "immediate", title = "Need immediate attention" },
		{ key = "attention", title = "Need attention" },
		{ key = "in_progress", title = "In progress" },
		{ key = "done", title = "Done today" },
	}

	for _, group in ipairs(groups) do
		local added = false
		for _, item in ipairs(state.items) do
			if item.attention == group.key then
				if not added then
					table.insert(lines, group.title)
					table.insert(lines, string.rep("─", #group.title))
					added = true
				end
				add_item(lines, line_items, item)
			end
		end
	end

	if #state.items == 0 then
		table.insert(lines, "No items need your attention.")
	end

	return lines, line_items
end

local function render_window()
	if not state.buf or not vim.api.nvim_buf_is_valid(state.buf) then
		state.buf = vim.api.nvim_create_buf(false, true)
	end

	local lines, line_items = render_lines()
	state.line_items = line_items

	vim.bo[state.buf].modifiable = true
	vim.api.nvim_buf_set_lines(state.buf, 0, -1, false, lines)
	vim.bo[state.buf].modifiable = false
	vim.bo[state.buf].buftype = "nofile"
	vim.bo[state.buf].bufhidden = "wipe"
	vim.bo[state.buf].swapfile = false
	vim.bo[state.buf].filetype = "radar"
end

local function ensure_window()
	render_window()
	if is_open() then
		vim.api.nvim_set_current_win(state.win)
		return
	end

	local width = math.min(config.width, vim.o.columns - 4)
	local height = math.min(config.height, vim.o.lines - 4)
	local row = math.floor((vim.o.lines - height) / 2)
	local col = math.floor((vim.o.columns - width) / 2)

	state.win = vim.api.nvim_open_win(state.buf, true, {
		relative = "editor",
		style = "minimal",
		border = "rounded",
		title = " Radar ",
		title_pos = "center",
		width = width,
		height = height,
		row = row,
		col = col,
	})

	vim.wo[state.win].wrap = false
	vim.wo[state.win].cursorline = true

	vim.keymap.set("n", "q", close_window, { buffer = state.buf, silent = true })
	vim.keymap.set("n", "<Esc>", close_window, { buffer = state.buf, silent = true })
	vim.keymap.set("n", "r", function()
		M.refresh(function()
			render_window()
		end)
	end, { buffer = state.buf, silent = true })
	vim.keymap.set("n", "<CR>", function()
		local line = vim.api.nvim_win_get_cursor(0)[1]
		local item = state.line_items[line]
		if item then
			open_url(item.url)
		end
	end, { buffer = state.buf, silent = true })
end

local function refresh_statusline()
	vim.cmd("redrawstatus")
	local ok, lualine = pcall(require, "lualine")
	if ok then
		lualine.refresh({ place = { "statusline" } })
	end
end

local function fetch(method, cb, retried)
	run({ config.radar_cmd, method }, function(result)
		if result.code ~= 0 then
			start_daemon()
			if not retried then
				vim.defer_fn(function()
					fetch(method, cb, true)
				end, 300)
				return
			end
			if cb then
				cb(false)
			end
			return
		end

		local response = decode_json(result.stdout)
		if not response or not response.ok then
			if cb then
				cb(false)
			end
			return
		end

		state.summary = response.summary or state.summary
		if response.items then
			notify_new_items(response.items)
			state.items = response.items
		end
		refresh_statusline()
		if is_open() then
			render_window()
		end
		if cb then
			cb(true)
		end
	end)
end

function M.refresh(cb)
	fetch("refresh", cb)
end

function M.load(cb)
	fetch("items", cb)
end

function M.statusline()
	local s = state.summary
	return string.format("%s%d %s%d %s%d %s%d", config.icons.immediate, s.immediate or 0, config.icons.attention, s.attention or 0, config.icons.in_progress, s.in_progress or 0, config.icons.done, s.done or 0)
end

function M.open()
	ensure_window()
	fetch("items", function()
		render_window()
	end)
end

function M.setup(opts)
	config = vim.tbl_deep_extend("force", config, opts or {})

	vim.api.nvim_create_user_command("Radar", M.open, {})
	vim.api.nvim_create_user_command("RadarRefresh", function()
		M.refresh()
	end, {})
	vim.api.nvim_create_user_command("RadarStart", start_daemon, {})
	vim.api.nvim_create_user_command("RadarStop", stop_daemon, {})
	vim.api.nvim_create_user_command("RadarRestart", restart_daemon, {})

	M.load()

	state.timer = vim.loop.new_timer()
	state.timer:start(
		config.refresh_ms,
		config.refresh_ms,
		vim.schedule_wrap(function()
			M.load()
		end)
	)
end

return M
