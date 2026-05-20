package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	mcp "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewInstanceManagerServer creates an MCP server exposing instance lifecycle tools.
func NewInstanceManagerServer(client *SimulatorClient) *server.MCPServer {
	s := server.NewMCPServer(
		"IEC104 Instance Manager",
		"1.0.0",
		server.WithLogging(),
	)

	// list_instances
	s.AddTool(mcp.NewTool("list_instances",
		mcp.WithDescription("列出所有已配置的模拟器实例"),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ListInstances()
	}))

	// get_instance
	s.AddTool(mcp.NewTool("get_instance",
		mcp.WithDescription("获取单个实例的详细配置信息"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.GetInstance(getStringArg(args, "instance_id"))
	}))

	// create_instance
	s.AddTool(mcp.NewTool("create_instance",
		mcp.WithDescription("创建新的模拟器实例"),
		mcp.WithString("config", mcp.Description("实例配置 JSON 字符串（name, port, xlsx_file等）")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.CreateInstance(json.RawMessage(getStringArg(args, "config")))
	}))

	// update_instance
	s.AddTool(mcp.NewTool("update_instance",
		mcp.WithDescription("更新已有实例的配置"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithString("config", mcp.Description("更新后的实例配置 JSON")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.UpdateInstance(getStringArg(args, "instance_id"), json.RawMessage(getStringArg(args, "config")))
	}))

	// delete_instance
	s.AddTool(mcp.NewTool("delete_instance",
		mcp.WithDescription("删除实例"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.DeleteInstance(getStringArg(args, "instance_id"))
	}))

	// start_instance
	s.AddTool(mcp.NewTool("start_instance",
		mcp.WithDescription("启动指定实例"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.StartInstance(getStringArg(args, "instance_id"))
	}))

	// stop_instance
	s.AddTool(mcp.NewTool("stop_instance",
		mcp.WithDescription("停止指定实例"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.StopInstance(getStringArg(args, "instance_id"))
	}))

	// restart_instance
	s.AddTool(mcp.NewTool("restart_instance",
		mcp.WithDescription("重启指定实例"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.RestartInstance(getStringArg(args, "instance_id"))
	}))

	// get_server_status
	s.AddTool(mcp.NewTool("get_server_status",
		mcp.WithDescription("获取模拟器全局状态，包括所有实例的运行数/总数"),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.GetServerStatus()
	}))

	// list_files
	s.AddTool(mcp.NewTool("list_files",
		mcp.WithDescription("列出 config 目录下所有 .xlsx 点表文件"),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ListFiles()
	}))

	// get_protocols
	s.AddTool(mcp.NewTool("get_protocols",
		mcp.WithDescription("查询模拟器支持的协议类型（如 IEC104、Modbus TCP）"),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.GetProtocols()
	}))

	return s
}

// NewDataInterfaceServer creates an MCP server exposing point data tools.
func NewDataInterfaceServer(client *SimulatorClient) *server.MCPServer {
	s := server.NewMCPServer(
		"IEC104 Data Interface",
		"1.0.0",
		server.WithLogging(),
	)

	// list_points
	s.AddTool(mcp.NewTool("list_points",
		mcp.WithDescription("列出实例的所有测点及其当前值，按 AI 优先 + IOA 升序排列"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ListPoints(getStringArg(args, "instance_id"))
	}))

	// read_point
	s.AddTool(mcp.NewTool("read_point",
		mcp.WithDescription("读取单个测点的当前值"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ReadPoint(getStringArg(args, "instance_id"), getUint32Arg(args, "ioa"))
	}))

	// read_points
	s.AddTool(mcp.NewTool("read_points",
		mcp.WithDescription("批量读取多个测点的当前值。不传 ioas 则返回全部。"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithArray("ioas",
			mcp.Description("可选！指定要读取的 IOA 列表，不传则返回全部测点"),
			mcp.WithNumberItems(),
		),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		instID := getStringArg(args, "instance_id")
		raw, err := c.ListPoints(instID)
		if err != nil {
			return nil, err
		}
		ioas := getStringArray(args, "ioas")
		if len(ioas) == 0 {
			return raw, nil
		}
		// Filter: parse the points list and return only matching IOAs
		var allPoints struct {
			Points []map[string]any `json:"points"`
		}
		if err := json.Unmarshal(raw, &allPoints); err != nil {
			return nil, err
		}
		ioaSet := make(map[string]bool, len(ioas))
		for _, ioa := range ioas {
			ioaSet[ioa] = true
		}
		filtered := make([]map[string]any, 0)
		for _, p := range allPoints.Points {
			ioaStr := fmt.Sprintf("%v", p["ioa"])
			if ioaSet[ioaStr] {
				filtered = append(filtered, p)
			}
		}
			rawResult, err := json.Marshal(map[string]any{"points": filtered})
		if err != nil {
			return nil, err
		}
		return json.RawMessage(rawResult), nil
	}))

	// write_point
	s.AddTool(mcp.NewTool("write_point",
		mcp.WithDescription("写入单个测点的值。AI 用 value(float)，DI 用 bool_value，PI 用 int_value。"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
		mcp.WithNumber("value",
			mcp.Description("浮点数值（AI 遥测使用），DI 也可用（非零=true）"),
		),
		mcp.WithBoolean("bool_value",
			mcp.Description("布尔值（DI 遥信使用）"),
		),
		mcp.WithNumber("int_value",
			mcp.Description("整数值（PI 遥脉使用）"),
		),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		instID := getStringArg(args, "instance_id")
		ioa := getUint32Arg(args, "ioa")
		body := buildPointBody(args)
		return c.WritePoint(instID, ioa, body)
	}))

	// write_points ⭐ 核心
	s.AddTool(mcp.NewTool("write_points",
		mcp.WithDescription("【核心】批量写入多个测点的值。一次调用写入多个 IOA，模拟真实设备同一时刻上报数据。这是自动化测试的关键接口。"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithArray("points",
			mcp.Description("要写入的测点列表，每个元素包含 {ioa, value?, bool_value?, int_value?}"),
			mcp.Items("object"),
			mcp.Required(),
		),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		instID := getStringArg(args, "instance_id")
		pointsArg, ok := args["points"].([]any)
		if !ok || len(pointsArg) == 0 {
			return nil, fmt.Errorf("points array is required and must not be empty")
		}
		// Convert each point to proper JSON based on type
		type pointEntry struct {
			IOA       uint32   `json:"ioa"`
			Value     *float64 `json:"value,omitempty"`
			BoolValue *bool    `json:"bool_value,omitempty"`
			IntValue  *int32   `json:"int_value,omitempty"`
		}
		points := make([]pointEntry, 0, len(pointsArg))
		for _, p := range pointsArg {
			pMap, ok := p.(map[string]any)
			if !ok {
				continue
			}
			entry := pointEntry{IOA: getUint32Arg(pMap, "ioa")}
			if v, exists := pMap["value"]; exists {
				fv, _ := v.(float64)
				entry.Value = &fv
			}
			if v, exists := pMap["bool_value"]; exists {
				bv, _ := v.(bool)
				entry.BoolValue = &bv
			}
			if v, exists := pMap["int_value"]; exists {
				iv, _ := v.(float64)
				i32 := int32(iv)
				entry.IntValue = &i32
			}
			points = append(points, entry)
		}
		body, err := json.Marshal(map[string]any{"points": points})
		if err != nil {
			return nil, fmt.Errorf("marshal points: %w", err)
		}
		return c.WritePoints(instID, body)
	}))

	// config_auto_change
	s.AddTool(mcp.NewTool("config_auto_change",
		mcp.WithDescription("配置测点的自动变化策略。支持: increment(递增), random(随机), csv(回放), max(取大), min(取小), soc(SOC), energy(电量), aofollow(AO关联), apiupdate(接口更新), manual(手动), custom(自定义公式)"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
		mcp.WithString("strategy", mcp.Description("策略类型: increment/random/csv/max/min/soc/energy/aofollow/apiupdate/manual/custom")),
		mcp.WithBoolean("enabled", mcp.Description("是否启用"), ),
		mcp.WithString("params", mcp.Description("策略参数 JSON 字符串，如 {\"start_value\":0,\"step\":1,\"period_ms\":1000,\"max_value\":100}")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		instID := getStringArg(args, "instance_id")
		ioa := getUint32Arg(args, "ioa")
		strategy := getStringArg(args, "strategy")
		enabled := getBoolArg(args, "enabled")

		paramsStr := getStringArg(args, "params")
		params := json.RawMessage("{}")
		if paramsStr != "" {
			params = json.RawMessage(paramsStr)
		}

		body, err := json.Marshal(map[string]any{
			"strategy": strategy,
			"enabled":  enabled,
			"params":   params,
		})
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		return c.ConfigAutoChange(instID, ioa, body)
	}))

	// get_auto_change
	s.AddTool(mcp.NewTool("get_auto_change",
		mcp.WithDescription("查看测点的自动变化配置"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.GetAutoChange(getStringArg(args, "instance_id"), getUint32Arg(args, "ioa"))
	}))

	// delete_auto_change
	s.AddTool(mcp.NewTool("delete_auto_change",
		mcp.WithDescription("删除测点的自动变化配置"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.DeleteAutoChange(getStringArg(args, "instance_id"), getUint32Arg(args, "ioa"))
	}))

	// export_auto_changes
	s.AddTool(mcp.NewTool("export_auto_changes",
		mcp.WithDescription("导出实例所有自动变化配置为 CSV 表格"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ExportAutoChanges(getStringArg(args, "instance_id"))
	}))

	// import_auto_changes
	s.AddTool(mcp.NewTool("import_auto_changes",
		mcp.WithDescription("从 CSV 内容导入自动变化配置"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithString("csv_content", mcp.Description("CSV 内容，包含信息体地址,测点名称,自动变化模式,A~G参数列")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ImportAutoChanges(getStringArg(args, "instance_id"), getStringArg(args, "csv_content"))
	}))

	// upload_csv
	s.AddTool(mcp.NewTool("upload_csv",
		mcp.WithDescription("上传 CSV 时间序列文件，用于 CSV 回放策略"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithString("csv_content", mcp.Description("CSV 文件内容")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.UploadCSV(getStringArg(args, "instance_id"), getStringArg(args, "csv_content"))
	}))

	// upload_file — 上传 xlsx 点表文件
	s.AddTool(mcp.NewTool("upload_file",
		mcp.WithDescription("上传 .xlsx 点表文件到模拟器 config 目录。文件内容需 base64 编码。"),
		mcp.WithString("filename", mcp.Description("文件名，如 \"固定验证-关口表.xlsx\"")),
		mcp.WithString("content_base64", mcp.Description("文件内容的 base64 编码")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		filename := getStringArg(args, "filename")
		content := getStringArg(args, "content_base64")
		if filename == "" || content == "" {
			return nil, fmt.Errorf("filename and content_base64 are required")
		}
		return c.UploadFile(filename, content)
	}))

	// export_points_csv — 导出测点实时数据为 CSV
	s.AddTool(mcp.NewTool("export_points_csv",
		mcp.WithDescription("导出实例所有测点实时数据为 CSV 格式（信息体地址/名称/类型/值/时间）"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		return c.ExportPointsCSV(getStringArg(args, "instance_id"))
	}))

	// batch_config_auto_change
	s.AddTool(mcp.NewTool("batch_config_auto_change",
		mcp.WithDescription("批量配置多个测点的自动变化策略。一次调用为多个 IOA 应用同一策略配置。"),
		mcp.WithString("instance_id", mcp.Description("实例ID")),
		mcp.WithArray("ioas",
			mcp.Description("要配置的 IOA 列表"),
			mcp.WithNumberItems(),
			mcp.Required(),
		),
		mcp.WithString("strategy", mcp.Description("策略类型: increment/random/csv/max/min/soc/energy/aofollow/apiupdate/manual/custom"), mcp.Required()),
		mcp.WithBoolean("enabled", mcp.Description("是否启用"), mcp.Required()),
		mcp.WithString("params", mcp.Description("策略参数 JSON 字符串")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		instID := getStringArg(args, "instance_id")
		ioas := args["ioas"].([]any)
		ioaNums := make([]uint32, 0, len(ioas))
		for _, v := range ioas {
			switch n := v.(type) {
			case float64:
				ioaNums = append(ioaNums, uint32(n))
			case string:
				if n2, err := strconv.ParseUint(n, 10, 32); err == nil {
					ioaNums = append(ioaNums, uint32(n2))
				}
			}
		}
		if len(ioaNums) == 0 {
			return nil, fmt.Errorf("ioas array is required and must not be empty")
		}

		paramsStr := getStringArg(args, "params")
		params := json.RawMessage("{}")
		if paramsStr != "" {
			params = json.RawMessage(paramsStr)
		}

		body, err := json.Marshal(map[string]any{
			"ioas":   ioaNums,
			"config": map[string]any{
				"strategy": getStringArg(args, "strategy"),
				"enabled":  getBoolArg(args, "enabled"),
				"params":   params,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("marshal config: %w", err)
		}
		return c.BatchConfigAutoChange(instID, body)
	}))

	// update_qds
	s.AddTool(mcp.NewTool("update_qds",
		mcp.WithDescription("更新测点的品质描述 QDS（invalid/not_topical/substituted/overflow/blocked）。传统模式 API。"),
		mcp.WithNumber("ioa", mcp.Description("信息体地址")),
		mcp.WithBoolean("invalid", mcp.Description("无效标志")),
		mcp.WithBoolean("not_topical", mcp.Description("非当前标志")),
		mcp.WithBoolean("substituted", mcp.Description("替代标志")),
		mcp.WithBoolean("overflow", mcp.Description("溢出标志")),
		mcp.WithBoolean("blocked", mcp.Description("闭锁标志")),
	), toolHandler(client, func(c *SimulatorClient, args map[string]any) (any, error) {
		ioa := getUint32Arg(args, "ioa")
		body := map[string]any{}
		if v, exists := args["invalid"]; exists {
			body["invalid"] = v
		}
		if v, exists := args["not_topical"]; exists {
			body["not_topical"] = v
		}
		if v, exists := args["substituted"]; exists {
			body["substituted"] = v
		}
		if v, exists := args["overflow"]; exists {
			body["overflow"] = v
		}
		if v, exists := args["blocked"]; exists {
			body["blocked"] = v
		}
		raw, _ := json.Marshal(body)
		return c.UpdateQDS(ioa, raw)
	}))

	return s
}

func toolHandler(client *SimulatorClient, fn func(*SimulatorClient, map[string]any) (any, error)) func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		args := req.GetArguments()
		result, err := fn(client, args)
		if err != nil {
			return mcp.NewToolResultText(fmt.Sprintf("错误: %v", err)), nil
		}
		var pretty bytes.Buffer
		if raw, ok := result.(json.RawMessage); ok {
			if err := json.Indent(&pretty, []byte(raw), "", "  "); err == nil {
				return mcp.NewToolResultText(pretty.String()), nil
			}
		}
		return mcp.NewToolResultText(fmt.Sprintf("%v", result)), nil
	}
}

func buildPointBody(args map[string]any) json.RawMessage {
	body := make(map[string]any)
	if v, exists := args["value"]; exists {
		body["value"] = v
	}
	if v, exists := args["bool_value"]; exists {
		body["bool_value"] = v
	}
	if v, exists := args["int_value"]; exists {
		body["int_value"] = v
	}
	raw, _ := json.Marshal(body)
	return raw
}
