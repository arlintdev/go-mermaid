package theme

// palettes maps theme names to palettes. Unknown names fall back to
// default. neutral, forest and base set only the shared colors; For fills
// the rest from default.
var palettes = map[string]Palette{
	"default": defaultPalette,
	"dark":    darkPalette,
	"neutral": {
		Background:      "#ffffff",
		NodeFill:        "#eeeeee",
		NodeStroke:      "#999999",
		Text:            "#222222",
		Edge:            "#555555",
		ClusterFill:     "#f4f4f4",
		ClusterStroke:   "#999999",
		LabelBackground: "#ececec",
	},
	"forest": {
		Background:      "#ffffff",
		NodeFill:        "#cde498",
		NodeStroke:      "#13540c",
		Text:            "#13540c",
		Edge:            "#3a7a2a",
		ClusterFill:     "#f2f9e8",
		ClusterStroke:   "#6eaa49",
		LabelBackground: "#e8f2df",
	},
	"base": {
		Background:      "#ffffff",
		NodeFill:        "#e8e8e8",
		NodeStroke:      "#666666",
		Text:            "#1a1a1a",
		Edge:            "#444444",
		ClusterFill:     "#f4f4f4",
		ClusterStroke:   "#888888",
		LabelBackground: "#ececec",
	},
}

// defaultPalette is Mermaid's default theme.
var defaultPalette = Palette{
	Background:      "#ffffff",
	NodeFill:        "#ECECFF",
	NodeStroke:      "#9370DB",
	Text:            "#333333",
	Edge:            "#333333",
	ClusterFill:     "#ffffde",
	ClusterStroke:   "#aaaa33",
	LabelBackground: "#e8e8e8",

	NoteFill:      "#fff5ad",
	NoteStroke:    "#aaaa33",
	NoteText:      "#333333",
	RelationLabel: "#e8e8e8",

	Sequence: SequenceColors{ActivationFill: "#f4f4f4", ActivationStroke: "#666666", NumberText: "#ffffff"},
	Gantt: GanttColors{
		TaskFill: "#8a90dd", TaskStroke: "#534fbc", TaskText: "#ffffff",
		ActiveFill: "#bfc7ff", ActiveText: "#000000",
		DoneFill: "#d3d3d3", DoneStroke: "#808080", DoneText: "#000000",
		CritFill: "#ff0000", CritStroke: "#ff8888",
		ExcludeFill: "#eeeeee", Grid: "#d3d3d3", Marker: "#000080",
		Bands: []Band{{"#6666ff", 0.1}, {"#ffffff", 0.2}, {"#fff400", 0.2}, {"#ffffff", 0.2}},
	},
	Git: GitColors{
		Lanes: []GitLane{
			{"#0000ec", "#ffffff", "#131300"},
			{"#dede00", "#000000", "#0000a1"},
			{"#9eec00", "#000000", "#310093"},
			{"#0076ec", "#ffffff", "#934900"},
			{"#00ecec", "#000000", "#930000"},
			{"#00ec76", "#000000", "#930049"},
			{"#ec00ec", "#000000", "#009300"},
			{"#ec0000", "#000000", "#009393"},
		},
		Mark:    "#ffffff",
		IDFill:  "#ffffde",
	},
	Journey: JourneyColors{
		Sections:    []string{"#ececff", "#ffffde", "#ffecfe", "#deffe0", "#ecfffe", "#ffdee0", "#ffefec", "#defbff"},
		Actors:      []string{"#8fbc8f", "#7cfc00", "#00ffff", "#20b2aa", "#b0e0e6", "#ffffe0"},
		ActorStroke: "#000000",
		Stroke:      "#666666",
		FaceFill:    "#fff8dc",
		FaceStroke:  "#999999",
		FaceFeature: "#666666",
	},
	Mindmap: MindmapColors{
		RootFill: "#0000ec", RootText: "#ffffff",
		Fills: []string{"#ffff78", "#d7ff86", "#c286ff", "#ff86ff", "#ff86c2", "#ff8686", "#ffc286", "#c2ff86", "#86ffc2", "#86ffff", "#86c2ff"},
		Edges: []string{"#ffff78", "#d7ff86", "#c286ff", "#ff86ff", "#ff86c2", "#ff8686", "#ffc286", "#c2ff86", "#86ffc2", "#86ffff", "#86c2ff"},
		Lines: []string{"#ababff", "#d0b9ff", "#dcffb9", "#b9ffb9", "#b9ffdc", "#b9ffff", "#b9dcff", "#dcb9ff", "#ffb9dc", "#ffb9b9", "#ffdcb9"},
		Text:  "#000000",
	},
	Pie: PieColors{
		Slices: []string{
			"#ececff", "#ffffde", "#b5ff20", "#b9b9ff", "#ffffab", "#9dec00",
			"#ffb9ff", "#b9ffff", "#ffecec", "#ff86ff", "#86ffff", "#ffb9b9",
		},
		Stroke: "#000000",
	},
	// Mermaid's default colour scale, from hues 240, 60, 80, 270, 300, 330,
	// 0, 30, 90, 150, 180 and 210.
	Timeline: []TimelineColor{
		{"#8686ff", "#6060b8", "#bcbcff", "#1f1f1f"},
		{"#ffff78", "#b8b856", "#ffffb5", "#1f1f1f"},
		{"#d7ff86", "#9bb860", "#e9ffbc", "#1f1f1f"},
		{"#c286ff", "#8c60b8", "#debcff", "#1f1f1f"},
		{"#ff86ff", "#b860b8", "#ffbcff", "#1f1f1f"},
		{"#ff86c2", "#b8608c", "#ffbcde", "#1f1f1f"},
		{"#ff8686", "#b86060", "#ffbcbc", "#1f1f1f"},
		{"#ffc286", "#b88c60", "#ffdebc", "#1f1f1f"},
		{"#c2ff86", "#8cb860", "#deffbc", "#1f1f1f"},
		{"#86ffc2", "#60b88c", "#bcffde", "#1f1f1f"},
		{"#86ffff", "#60b8b8", "#bcffff", "#1f1f1f"},
		{"#86c2ff", "#608cb8", "#bcdeff", "#1f1f1f"},
	},
	XYChart: XYChartColors{
		Series: []string{"#ececff", "#8493a6", "#ffb6c1", "#c4a000", "#fcfc7f", "#f5deb3", "#87ceeb", "#ffe4e1", "#e6e6fa", "#90ee90"},
	},
	Kanban: KanbanColors{
		Columns:    []string{"#ffffab", "#e8ffb9", "#dcb9ff", "#ffb9ff", "#ffb9dc", "#ffb9b9", "#ffdcb9", "#dcffb9", "#b9ffdc", "#b9ffff", "#b9dcff"},
		ColumnText: "#333333",
		CardFill:   "#ffffff", CardStroke: "#9370db",
		VeryHigh: "#ff0000", High: "#ffa500", Low: "#0000ff", VeryLow: "#add8e6",
	},
	Packet: PacketColors{Fill: "#efefef", Stroke: "#000000", Text: "#000000"},
	Radar: RadarColors{
		Curves: []string{"#8686ff", "#ffff78", "#d7ff86", "#c286ff", "#ff86ff", "#ff86c2", "#ff8686", "#ffc286", "#c2ff86", "#86ffc2", "#86ffff", "#86c2ff"},
		Grid:   "#dedede",
		Axis:   "#333333",
	},
	// d3's Tableau10 scheme, which Mermaid colours sankey nodes with.
	Sankey: SankeyColors{Nodes: []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab"}},
	C4: C4Colors{
		Person:            C4Look{"#08427b", "#073b6f", "#ffffff"},
		ExternalPerson:    C4Look{"#686868", "#8a8a8a", "#ffffff"},
		System:            C4Look{"#1168bd", "#3c7fc0", "#ffffff"},
		ExternalSystem:    C4Look{"#999999", "#8a8a8a", "#ffffff"},
		Container:         C4Look{"#438dd5", "#3c7fc0", "#ffffff"},
		ExternalContainer: C4Look{"#b3b3b3", "#a6a6a6", "#ffffff"},
		Component:         C4Look{"#85bbf0", "#78a8d8", "#000000"},
		ExternalComponent: C4Look{"#cccccc", "#bfbfbf", "#000000"},
		Line:              "#444444",
	},
}

// darkPalette is drawn for a dark page: the default theme's hues, dark
// fills under light text, and lines light enough to read on the page.
var darkPalette = Palette{
	Background:      "#1e1e1e",
	NodeFill:        "#2b2b40",
	NodeStroke:      "#8888bb",
	Text:            "#e6e6e6",
	Edge:            "#bbbbbb",
	ClusterFill:     "#2a2a33",
	ClusterStroke:   "#77775a",
	LabelBackground: "#3a3a3a",

	NoteFill:      "#4a4630",
	NoteStroke:    "#8a8350",
	NoteText:      "#eeeadc",
	RelationLabel: "#3a3a3a",

	Sequence: SequenceColors{ActivationFill: "#3a3a4e", ActivationStroke: "#8888bb", NumberText: "#1e1e1e"},
	Gantt: GanttColors{
		TaskFill: "#4b4f9a", TaskStroke: "#8a90dd", TaskText: "#ffffff",
		ActiveFill: "#2e3466", ActiveText: "#e6e6e6",
		DoneFill: "#4a4a4a", DoneStroke: "#8a8a8a", DoneText: "#e6e6e6",
		CritFill: "#b02a2a", CritStroke: "#ff8888",
		ExcludeFill: "#2c2c2c", Grid: "#4a4a4a", Marker: "#5fb8ff",
		Bands: []Band{{"#6666ff", 0.12}, {"#ffffff", 0.03}, {"#fff400", 0.07}, {"#ffffff", 0.03}},
	},
	Git: GitColors{
		Lanes: []GitLane{
			{"#4a4ae0", "#ffffff", "#e6e699"},
			{"#cfcf3c", "#1e1e1e", "#9999e6"},
			{"#9ccc3c", "#1e1e1e", "#b299e6"},
			{"#3c8cdd", "#ffffff", "#e6bf99"},
			{"#3ccfcf", "#1e1e1e", "#e69999"},
			{"#3ccf86", "#1e1e1e", "#e699bf"},
			{"#cf4ccf", "#1e1e1e", "#99e699"},
			{"#dd4a4a", "#1e1e1e", "#99e6e6"},
		},
		Mark:    "#1e1e1e",
		IDFill:  "#474949",
	},
	Journey: JourneyColors{
		Sections:    []string{"#30305a", "#5a5a30", "#5a3057", "#305a33", "#305a57", "#5a3033", "#5a3730", "#30545a"},
		Actors:      []string{"#8fbc8f", "#7cfc00", "#00ffff", "#20b2aa", "#b0e0e6", "#ffffe0"},
		ActorStroke: "#1e1e1e",
		Stroke:      "#8a8a8a",
		FaceFill:    "#fff8dc",
		FaceStroke:  "#999999",
		FaceFeature: "#666666",
	},
	Mindmap: MindmapColors{
		RootFill: "#3b3bb8", RootText: "#ffffff",
		Fills: []string{"#76762d", "#5e762d", "#512d76", "#762d76", "#762d51", "#762d2d", "#76512d", "#51762d", "#2d7651", "#2d7676", "#2d5176"},
		Edges: []string{"#a1a136", "#7da136", "#6b36a1", "#a136a1", "#a1366b", "#a13636", "#a16b36", "#6ba136", "#36a16b", "#36a1a1", "#366ba1"},
		Lines: []string{"#d1d147", "#a4d147", "#8c47d1", "#d147d1", "#d1478c", "#d14747", "#d18c47", "#8cd147", "#47d18c", "#47d1d1", "#478cd1"},
		Text:  "#f0f0f0",
	},
	Pie: PieColors{
		Slices: []string{
			"#6a6ad0", "#c2b84a", "#86c23a", "#9a7ae0", "#d6b83a", "#5aa830",
			"#c26ac2", "#4ab8b8", "#d07a7a", "#d65ad6", "#3ac2c2", "#e08a6a",
		},
		Stroke: "#1e1e1e",
	},
	Timeline: []TimelineColor{
		{"#2e2e7a", "#4747d1", "#29294c", "#eeeeee"},
		{"#7a7a2e", "#d1d147", "#4c4c29", "#eeeeee"},
		{"#617a2e", "#a4d147", "#414c29", "#eeeeee"},
		{"#542e7a", "#8c47d1", "#3b294c", "#eeeeee"},
		{"#7a2e7a", "#d147d1", "#4c294c", "#eeeeee"},
		{"#7a2e54", "#d1478c", "#4c293b", "#eeeeee"},
		{"#7a2e2e", "#d14747", "#4c2929", "#eeeeee"},
		{"#7a542e", "#d18c47", "#4c3b29", "#eeeeee"},
		{"#547a2e", "#8cd147", "#3b4c29", "#eeeeee"},
		{"#2e7a54", "#47d18c", "#294c3b", "#eeeeee"},
		{"#2e7a7a", "#47d1d1", "#294c4c", "#eeeeee"},
		{"#2e547a", "#478cd1", "#293b4c", "#eeeeee"},
	},
	// Mermaid's dark xychart palette.
	XYChart: XYChartColors{
		Series: []string{"#3498db", "#2ecc71", "#e74c3c", "#f1c40f", "#bdc3c7", "#ffffff", "#34495e", "#9b59b6", "#1abc9c", "#e67e22"},
	},
	Kanban: KanbanColors{
		Columns:    []string{"#50502b", "#43502b", "#3d2b50", "#502b50", "#502b3d", "#502b2b", "#503d2b", "#3d502b", "#2b503d", "#2b5050", "#2b3d50"},
		ColumnText: "#e6e6e6",
		CardFill:   "#262626", CardStroke: "#8888bb",
		VeryHigh: "#ff5c5c", High: "#ffae3c", Low: "#5c8cff", VeryLow: "#add8e6",
	},
	Packet: PacketColors{Fill: "#333333", Stroke: "#cccccc", Text: "#e6e6e6"},
	Radar: RadarColors{
		Curves: []string{"#6a6ae0", "#dbdb57", "#afdb57", "#9857db", "#db57db", "#db5798", "#db5757", "#db9857", "#98db57", "#57db98", "#57dbdb", "#5798db"},
		Grid:   "#5a5a5a",
		Axis:   "#cccccc",
	},
	Sankey: SankeyColors{Nodes: []string{"#4e79a7", "#f28e2c", "#e15759", "#76b7b2", "#59a14f", "#edc949", "#af7aa1", "#ff9da7", "#9c755f", "#bab0ab"}},
	C4: C4Colors{
		Person:            C4Look{"#08427b", "#3c7fc0", "#ffffff"},
		ExternalPerson:    C4Look{"#686868", "#8a8a8a", "#ffffff"},
		System:            C4Look{"#1168bd", "#3c7fc0", "#ffffff"},
		ExternalSystem:    C4Look{"#6e6e6e", "#8a8a8a", "#ffffff"},
		Container:         C4Look{"#2f6fae", "#5c96d0", "#ffffff"},
		ExternalContainer: C4Look{"#5a5a5a", "#7a7a7a", "#ffffff"},
		Component:         C4Look{"#24507c", "#4d82b8", "#ffffff"},
		ExternalComponent: C4Look{"#4a4a4a", "#6a6a6a", "#ffffff"},
		Line:              "#aaaaaa",
	},
}
