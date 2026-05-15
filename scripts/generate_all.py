#!/usr/bin/env python3
"""
IEC104 点表生成器 — 从 Python 模板生成标准格式的 .xlsx 点表文件。
用于快速部署模拟器到新服务器。
"""
from __future__ import annotations

import openpyxl
import os
from typing import List, Dict

HEADERS = ["point-name", "point-number", "value-type", "point-type",
           "efficient", "base-value", "alias", "CtrlType"]


def _write_xlsx(path: str, points: List[Dict]):
    """写入标准格式 xlsx。"""
    wb = openpyxl.Workbook()
    ws = wb.active
    ws.title = "point"
    ws.append(HEADERS)
    for pt in points:
        ws.append([
            pt["name"], pt["ioa"], pt.get("value_type", "DOUBLE"),
            pt["type"], pt.get("efficient", "1"), pt.get("base", "0"),
            pt.get("alias", ""), pt.get("ctrl_type", ""),
        ])
    wb.save(path)
    print(f"  ✅ 已生成: {path} ({len(points)} 点)")


def generate_关口表(path: str):
    """关口表: 6 个 AI 点 (16385-16390)"""
    points = [
        {"name": "有功功率",     "ioa": 16385, "type": "AI", "alias": "METER.ActivePW"},
        {"name": "A相电流",     "ioa": 16386, "type": "AI", "alias": "METER.CurPh1"},
        {"name": "B相电流",     "ioa": 16387, "type": "AI", "alias": "METER.CurPh2"},
        {"name": "C相电流",     "ioa": 16388, "type": "AI", "alias": "METER.CurPh3"},
        {"name": "仪表频率",     "ioa": 16389, "type": "AI", "alias": "METER.Frequency"},
        {"name": "当月最大需量",  "ioa": 16390, "type": "AI", "alias": "METER.MaxDemandPW"},
    ]
    _write_xlsx(path, points)


def generate_储能(path: str):
    """储能: 8 AI (16385-16392) + 3 AO (25089-25091) = 11 点"""
    points = [
        {"name": "充放电功率",      "ioa": 16385, "type": "AI", "alias": "BS.ActivePW"},
        {"name": "电池SOC",         "ioa": 16386, "type": "AI", "alias": "BS.Soc"},
        {"name": "电池SOH",         "ioa": 16387, "type": "AI", "alias": "BS.Soh"},
        {"name": "可控状态",        "ioa": 16388, "type": "AI", "alias": "BS.CtrlState"},
        {"name": "最大放电功率",     "ioa": 16389, "type": "AI", "alias": "BS.MaxDischargePower"},
        {"name": "最大充电功率",     "ioa": 16390, "type": "AI", "alias": "BS.MaxChargePower"},
        {"name": "放电截止SOC",     "ioa": 16391, "type": "AI", "alias": "BS.EndChargeSOC"},
        {"name": "充电截止SOC",     "ioa": 16392, "type": "AI", "alias": "BS.EndDischargeSOC"},
        {"name": "系统有功功率控制值", "ioa": 25089, "type": "AO", "alias": "BS.SysAPSetPoint"},
        {"name": "远程启机",        "ioa": 25090, "type": "AO", "alias": "BS.Start"},
        {"name": "远程关机",        "ioa": 25091, "type": "AO", "alias": "BS.Stop"},
    ]
    _write_xlsx(path, points)


def generate_光伏(path: str):
    """光伏: 2 AI (16385-16386) + 3 AO (25089-25091) = 5 点"""
    points = [
        {"name": "可控状态",        "ioa": 16385, "type": "AI", "alias": "INV.CtrlState"},
        {"name": "有功功率",        "ioa": 16386, "type": "AI", "alias": "INV.GenActivePW"},
        {"name": "限有功功率实际值",  "ioa": 25089, "type": "AO", "alias": "INV.LimitPower"},
        {"name": "远程关机",        "ioa": 25090, "type": "AO", "alias": "INV.Stop"},
        {"name": "远程启机",        "ioa": 25091, "type": "AO", "alias": "INV.Start"},
    ]
    _write_xlsx(path, points)


def generate_FCR(path: str):
    """FCR: 14 AI (16385-16398) + 11 AO (25089-25099) = 25 点"""
    points = [
        {"name": "ap_exp_percent_ulimit_value", "ioa": 16385, "type": "AI", "alias": "ap_exp_percent_ulimit_value"},
        {"name": "participate_dr",              "ioa": 16386, "type": "AI", "alias": "participate_dr"},
        {"name": "aggregator_status",           "ioa": 16387, "type": "AI", "alias": "aggregator_status"},
        {"name": "agc_status",                  "ioa": 16388, "type": "AI", "alias": "agc_status"},
        {"name": "fcr_p1_setpoint",             "ioa": 16389, "type": "AI", "alias": "fcr_p1_setpoint"},
        {"name": "fcr_p2_setpoint",             "ioa": 16390, "type": "AI", "alias": "fcr_p2_setpoint"},
        {"name": "fcr_p3_setpoint",             "ioa": 16391, "type": "AI", "alias": "fcr_p3_setpoint"},
        {"name": "fcr_p4_setpoint",             "ioa": 16392, "type": "AI", "alias": "fcr_p4_setpoint"},
        {"name": "fcr_p5_setpoint",             "ioa": 16393, "type": "AI", "alias": "fcr_p5_setpoint"},
        {"name": "fcr_p6_setpoint",             "ioa": 16394, "type": "AI", "alias": "fcr_p6_setpoint"},
        {"name": "fcr_p7-1_setpoint",           "ioa": 16395, "type": "AI", "alias": "fcr_p7-1_setpoint"},
        {"name": "fcr_p7-2_setpoint",           "ioa": 16396, "type": "AI", "alias": "fcr_p7-2_setpoint"},
        {"name": "fcr_p8_setpoint",             "ioa": 16397, "type": "AI", "alias": "fcr_p8_setpoint"},
        {"name": "fcr_mode_setpoint",           "ioa": 16398, "type": "AI", "alias": "fcr_mode_setpoint"},
        {"name": "fcr_p1_value",                "ioa": 25089, "type": "AO", "alias": "fcr_p1_value"},
        {"name": "fcr_p2_value",                "ioa": 25090, "type": "AO", "alias": "fcr_p2_value"},
        {"name": "fcr_p3_value",                "ioa": 25091, "type": "AO", "alias": "fcr_p3_value"},
        {"name": "fcr_p4_value",                "ioa": 25092, "type": "AO", "alias": "fcr_p4_value"},
        {"name": "fcr_p5_value",                "ioa": 25093, "type": "AO", "alias": "fcr_p5_value"},
        {"name": "fcr_p6_value",                "ioa": 25094, "type": "AO", "alias": "fcr_p6_value"},
        {"name": "fcr_p7-1_value",              "ioa": 25095, "type": "AO", "alias": "fcr_p7-1_value"},
        {"name": "fcr_p7-2_value",              "ioa": 25096, "type": "AO", "alias": "fcr_p7-2_value"},
        {"name": "fcr_p8_value",                "ioa": 25097, "type": "AO", "alias": "fcr_p8_value"},
        {"name": "fcr_mode_value",              "ioa": 25098, "type": "AO", "alias": "fcr_mode_value"},
        {"name": "bess_ap_target_value",        "ioa": 25099, "type": "AO", "alias": "bess_ap_target_value"},
    ]
    _write_xlsx(path, points)


def generate_all(output_dir: str = ".", prefix: str = "固定验证"):
    """生成所有标准点表。"""
    os.makedirs(output_dir, exist_ok=True)
    print(f"生成点表文件到: {output_dir}")
    print(f"文件名前缀: {prefix}")

    generate_关口表(os.path.join(output_dir, f"{prefix}-关口表.xlsx"))
    generate_储能(os.path.join(output_dir, f"{prefix}-储能.xlsx"))
    generate_光伏(os.path.join(output_dir, f"{prefix}-光伏.xlsx"))
    generate_FCR(os.path.join(output_dir, f"{prefix}-FCR.xlsx"))

    print(f"\n✅ 全部生成完成 ({output_dir})")


if __name__ == "__main__":
    import sys
    import argparse
    parser = argparse.ArgumentParser(description="IEC104 点表生成器")
    parser.add_argument("--output", "-o", default=".", help="输出目录")
    parser.add_argument("--prefix", "-p", default="固定验证", help="文件名前缀")
    args = parser.parse_args()
    generate_all(args.output, args.prefix)
