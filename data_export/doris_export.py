
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Doris 数据导出工具
用于从 Doris 数据库导出指定表的数据到 Parquet 文件

依赖安装:
    pip install pymysql pandas pyarrow sqlalchemy

使用方法:
    python doris_export.py --host <host> --port <port> --user <user> --password <password> \
                           --database <database> --table <table> --output <output_dir>

示例:
    python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \
                           --database test_db --table users --output ./export_data
"""

import argparse
import os
import sys
from datetime import datetime
from typing import Any, Optional
from urllib.parse import quote_plus

import pandas as pd
from sqlalchemy import create_engine, text
from sqlalchemy.exc import SQLAlchemyError


def create_doris_engine(
    host: str, port: int, user: str, password: str, database: Optional[str] = None
):
    """
    创建 Doris 数据库连接引擎

    Args:
        host: Doris FE 主机地址
        port: Doris FE 查询端口（默认 9030）
        user: 用户名
        password: 密码
        database: 默认数据库名（可选）

    Returns:
        SQLAlchemy Engine 对象
    """
    # 对密码进行 URL 编码，处理特殊字符
    encoded_password = quote_plus(password)

    # 构建连接字符串
    # Doris 使用 MySQL 协议
    if database:
        db_url = f"mysql+pymysql://{user}:{encoded_password}@{host}:{port}/{database}"
    else:
        db_url = f"mysql+pymysql://{user}:{encoded_password}@{host}:{port}"

    engine = create_engine(
        db_url,
        pool_pre_ping=True,  # 连接前 ping，自动处理断线重连
        pool_recycle=3600,  # 连接回收时间
    )
    return engine


def get_table_columns(engine, database: str, table: str):
    """
    获取表的列信息

    Args:
        engine: SQLAlchemy Engine
        database: 数据库名
        table: 表名

    Returns:
        列名列表
    """
    query = text(
        """
        SELECT COLUMN_NAME 
        FROM information_schema.COLUMNS 
        WHERE TABLE_SCHEMA = :database AND TABLE_NAME = :table
        ORDER BY ORDINAL_POSITION
    """
    )

    with engine.connect() as conn:
        result = conn.execute(query, {"database": database, "table": table})
        columns = [row[0] for row in result]

    return columns


def export_table_to_parquet(
    engine,
    database: str,
    table: str,
    output_dir: str,
    batch_size: Optional[int] = None,
    where_clause: Optional[str] = None,
    partition_column: Optional[str] = None,
):
    """
    将 Doris 表数据导出为 Parquet 文件

    Args:
        engine: SQLAlchemy Engine
        database: 数据库名
        table: 表名
        output_dir: 输出目录
        batch_size: 每批读取的行数
        where_clause: 可选的 WHERE 条件
        partition_column: 可选的分区列，用于按列值分目录存储

    Returns:
        导出的文件路径列表
    """
    # 确保输出目录存在
    os.makedirs(output_dir, exist_ok=True)

    # 构建查询 SQL
    base_query = f"SELECT * FROM `{database}`.`{table}`"
    if where_clause:
        base_query += f" WHERE {where_clause}"

    # 如果需要分区，先获取分区列的唯一值
    if partition_column:
        partition_query = (
            f"SELECT DISTINCT `{partition_column}` FROM `{database}`.`{table}`"
        )
        if where_clause:
            partition_query += f" WHERE {where_clause}"

        with engine.connect() as conn:
            partitions = pd.read_sql(partition_query, conn)[partition_column].tolist()

        exported_files = []
        for partition_value in partitions:
            # 构建分区目录
            partition_dir = os.path.join(
                output_dir, f"{partition_column}={partition_value}"
            )
            os.makedirs(partition_dir, exist_ok=True)

            # 构建分区查询
            partition_where = f"`{partition_column}` = '{partition_value}'"
            if where_clause:
                partition_where = f"({where_clause}) AND {partition_where}"

            partition_query_full = f"{base_query} WHERE {partition_where}"

            # 导出该分区数据
            files = _export_query_to_parquet(
                engine, partition_query_full, partition_dir, table, batch_size
            )
            exported_files.extend(files)

        return exported_files
    else:
        # 不分区，直接导出
        return _export_query_to_parquet(
            engine, base_query, output_dir, table, batch_size
        )


def _export_query_to_parquet(
    engine, query: str, output_dir: str, table: str, batch_size: Optional[int] = None
):
    """
    内部方法：将查询结果导出为 Parquet 文件

    Args:
        engine: SQLAlchemy Engine
        query: SQL 查询语句
        output_dir: 输出目录
        table: 表名（用于生成文件名）
        batch_size: 每批读取的行数，None 表示不分批（一次性读取）

    Returns:
        导出的文件路径列表
    """
    exported_files = []
    batch_num = 0
    total_rows = 0

    # 生成文件名
    timestamp = datetime.now().strftime("%Y%m%d_%H%M%S")

    if batch_size is None:
        # 不分批，一次性读取所有数据
        print(f"  使用不分批模式导出...")
        with engine.connect() as conn:
            df = pd.read_sql(query, conn)
            print(df)
            total_rows = len(df)

            filename = f"{table}_{timestamp}.parquet"
            filepath = os.path.join(output_dir, filename)

            df.to_parquet(filepath, engine="pyarrow", compression="snappy", index=False)
            exported_files.append(filepath)
            print(f"  导出完成: {total_rows} 行 -> {filename}")
    else:
        # 使用 server-side cursor 进行流式读取
        with engine.connect().execution_options(stream_results=True) as conn:
            # 分批读取数据
            for chunk in pd.read_sql(query, conn, chunksize=batch_size):
                batch_num += 1
                rows_in_chunk = len(chunk)
                total_rows += rows_in_chunk

                # 生成文件名：表名_时间戳_批次号.parquet
                filename = f"{table}_{timestamp}_batch{batch_num:04d}.parquet"
                filepath = os.path.join(output_dir, filename)

                # 保存为 Parquet 格式
                # 使用 snappy 压缩，兼容性好
                chunk.to_parquet(
                    filepath, engine="pyarrow", compression="snappy", index=False
                )

                exported_files.append(filepath)
                print(f"  批次 {batch_num}: {rows_in_chunk} 行 -> {filename}")

        print(f"\n导出完成！总共 {total_rows} 行，生成 {batch_num} 个文件")

    return exported_files


def get_table_info(engine, database: str, table: str) -> dict[str, Any]:
    """
    获取表的统计信息

    Args:
        engine: SQLAlchemy Engine
        database: 数据库名
        table: 表名

    Returns:
        包含行数、列数等信息的字典
    """
    info: dict[str, Any] = {"database": database, "table": table}

    try:
        # 获取列信息
        columns = get_table_columns(engine, database, table)
        info["columns"] = columns
        info["column_count"] = len(columns)

        # 获取行数（近似值）
        count_query = text(f"SELECT COUNT(*) FROM `{database}`.`{table}`")
        with engine.connect() as conn:
            result = conn.execute(count_query)
            info["row_count"] = result.scalar()

        # Doris 的 information_schema.TABLES 中获取表信息（TABLE_STATISTICS 表不存在）
        try:
            size_query = text(
                """
                SELECT 
                    ROUND(SUM(data_length) / 1024 / 1024, 2) as data_size_mb,
                    ROUND(SUM(index_length) / 1024 / 1024, 2) as index_size_mb
                FROM information_schema.TABLES 
                WHERE TABLE_SCHEMA = :database AND TABLE_NAME = :table
            """
            )
            with engine.connect() as conn:
                result = conn.execute(
                    size_query, {"database": database, "table": table}
                )
                row = result.fetchone()
                if row:
                    info["data_size_mb"] = row[0] or 0
                    info["index_size_mb"] = row[1] or 0
        except Exception:
            # 忽略表大小查询失败
            pass

    except Exception as e:
        print(f"警告: 获取表信息时出错: {e}")

    return info


def verify_parquet_file(filepath: str) -> dict[str, Any]:
    """
    验证 Parquet 文件的完整性和可读性

    Args:
        filepath: Parquet 文件路径

    Returns:
        验证结果字典，包含：
        - valid: 是否有效
        - error: 错误信息（如果无效）
        - row_count: 行数
        - column_count: 列数
        - columns: 列名列表
        - size_mb: 文件大小(MB)
    """
    import pyarrow.parquet as pq

    result: dict[str, Any] = {"filepath": filepath, "valid": False}

    try:
        # 检查文件是否存在
        if not os.path.exists(filepath):
            result["error"] = "文件不存在"
            return result

        # 尝试读取 Parquet 文件的元数据
        parquet_file = pq.ParquetFile(filepath)
        metadata = parquet_file.metadata

        result["valid"] = True
        result["row_count"] = metadata.num_rows
        result["column_count"] = metadata.num_columns
        result["row_groups"] = metadata.num_row_groups
        result["created_by"] = metadata.created_by

        # 获取列信息
        schema = parquet_file.schema_arrow
        result["columns"] = schema.names
        result["column_types"] = [str(field.type) for field in schema]

        # 尝试读取前几行验证数据可读性
        table = pq.read_table(filepath, columns=[schema.names[0]])
        sample_data = table.to_pandas().head(3)
        result["sample_readable"] = len(sample_data) > 0

        # 文件大小
        result["size_mb"] = round(os.path.getsize(filepath) / (1024 * 1024), 2)

    except Exception as e:
        result["error"] = str(e)

    return result


def verify_exported_files(filepaths: list[str]) -> bool:
    """
    验证所有导出的 Parquet 文件

    Args:
        filepaths: 文件路径列表

    Returns:
        是否全部验证通过
    """
    print("\n开始验证导出的文件...")

    all_valid = True
    total_rows = 0

    for filepath in filepaths:
        print(f"\n  验证: {os.path.basename(filepath)}")
        result = verify_parquet_file(filepath)

        if result["valid"]:
            print(f"    ✓ 有效")
            print(f"    行数: {result['row_count']:,}")
            print(f"    列数: {result['column_count']}")
            print(f"    大小: {result['size_mb']:.2f} MB")
            total_rows += result["row_count"]
        else:
            print(f"    ✗ 无效: {result.get('error', '未知错误')}")
            all_valid = False

    print(f"\n验证结果: {'全部通过' if all_valid else '存在无效文件'}")
    print(f"总行数: {total_rows:,}")

    return all_valid


def main():
    parser = argparse.ArgumentParser(
        description="Doris 数据导出工具 - 导出表数据为 Parquet 格式",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
示例:
  # 基本导出（分批模式，默认 batch-size=100000）
  python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \\
                         --database test_db --table users --output ./export

  # 不分批导出（一次性读取所有数据）
  python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \\
                         --database test_db --table users --output ./export --no-batch

  # 带 WHERE 条件导出
  python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \\
                         --database test_db --table orders --output ./export \\
                         --where "create_time >= '2024-01-01'"

  # 按列分区导出
  python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \\
                         --database test_db --table logs --output ./export \\
                         --partition-by date_column

  # 导出并验证
  python doris_export.py --host 127.0.0.1 --port 9030 --user root --password 123456 \\
                         --database test_db --table users --output ./export --verify

  # 单独验证已导出的文件
  python doris_export.py --verify-only ./export/users_*.parquet
        """,
    )

    # 连接参数
    parser.add_argument("--host", required=True, help="Doris FE 主机地址")
    parser.add_argument(
        "--port", type=int, default=9030, help="Doris FE 查询端口（默认: 9030）"
    )
    parser.add_argument("--user", "-u", required=True, help="用户名")
    parser.add_argument("--password", "-p", required=True, help="密码")
    parser.add_argument("--database", "-d", required=True, help="数据库名")

    # 导出参数
    parser.add_argument("--table", "-t", help="要导出的表名")
    parser.add_argument("--output", "-o", help="输出目录")
    parser.add_argument(
        "--batch-size", type=int, help="每批读取的行数（不指定则不分批，一次性导出）"
    )
    parser.add_argument("--where", help="可选的 WHERE 过滤条件")
    parser.add_argument(
        "--partition-by", help="按指定列分区导出（数据会按列值分目录存储）"
    )

    # 其他选项
    parser.add_argument(
        "--info-only", action="store_true", help="仅显示表信息，不导出数据"
    )
    parser.add_argument(
        "--no-confirm", action="store_true", help="大表导出时不提示确认"
    )
    parser.add_argument("--verify", action="store_true", help="导出后验证 Parquet 文件")
    parser.add_argument(
        "--verify-only",
        nargs="+",
        metavar="FILE",
        help="仅验证指定的 Parquet 文件（可指定多个文件或使用通配符）",
    )

    args = parser.parse_args()

    # 处理仅验证模式
    if args.verify_only:
        import glob

        files_to_verify = []
        for pattern in args.verify_only:
            matched = glob.glob(pattern)
            if matched:
                files_to_verify.extend(matched)
            else:
                files_to_verify.append(pattern)

        if not files_to_verify:
            print("错误: 未找到要验证的文件")
            sys.exit(1)

        all_valid = verify_exported_files(files_to_verify)
        sys.exit(0 if all_valid else 1)

    # 检查必需的导出参数
    if not args.table or not args.output:
        parser.error("导出模式需要 --table 和 --output 参数")

    # 创建输出目录
    os.makedirs(args.output, exist_ok=True)

    # 创建数据库连接
    print(f"正在连接到 Doris: {args.host}:{args.port}...")
    try:
        engine = create_doris_engine(
            host=args.host,
            port=args.port,
            user=args.user,
            password=args.password,
            database=args.database,
        )
        # 测试连接
        with engine.connect() as conn:
            conn.execute(text("SELECT 1"))
        print("连接成功！")
    except SQLAlchemyError as e:
        print(f"连接失败: {e}")
        sys.exit(1)

    # 获取表信息
    print(f"\n正在获取表 '{args.database}.{args.table}' 的信息...")
    table_info = get_table_info(engine, args.database, args.table)

    print(f"\n表信息:")
    print(f"  数据库: {table_info['database']}")
    print(f"  表名: {table_info['table']}")
    print(f"  列数: {table_info.get('column_count', 'N/A')}")
    print(f"  行数: {table_info.get('row_count', 'N/A'):,}")
    if "data_size_mb" in table_info:
        print(f"  数据大小: {table_info['data_size_mb']:.2f} MB")

    if args.info_only:
        print("\n列列表:")
        for col in table_info.get("columns", []):
            print(f"  - {col}")
        return

    # 确认导出（大表）
    row_count = table_info.get("row_count", 0)
    if isinstance(row_count, int) and row_count > 1000000 and not args.no_confirm:
        confirm = input(
            f"\n警告: 表包含 {row_count:,} 行数据，导出可能需要较长时间。\n是否继续? [y/N]: "
        )
        if confirm.lower() != "y":
            print("已取消导出")
            return

    # 执行导出
    print(f"\n开始导出数据到: {args.output}")
    if args.where:
        print(f"WHERE 条件: {args.where}")
    if args.partition_by:
        print(f"分区列: {args.partition_by}")
    print()

    try:
        exported_files = export_table_to_parquet(
            engine=engine,
            database=args.database,
            table=args.table,
            output_dir=args.output,
            batch_size=args.batch_size,
            where_clause=args.where,
            partition_column=args.partition_by,
        )

        print(f"\n导出成功！")
        print(f"文件列表:")
        for f in exported_files:
            size_mb = os.path.getsize(f) / (1024 * 1024)
            print(f"  - {f} ({size_mb:.2f} MB)")

        # 验证导出的文件（如果启用）
        if args.verify:
            verify_exported_files(exported_files)

    except Exception as e:
        print(f"\n导出失败: {e}")
        import traceback

        traceback.print_exc()
        sys.exit(1)
    finally:
        engine.dispose()


if __name__ == "__main__":
    # "numpy>=1.22,<2" "pandas==2.2.2"  "pyarrow<15"
    # pip install "SQLAlchemy=2.0.48" --no-deps
    main()

