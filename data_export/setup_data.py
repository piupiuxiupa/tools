#!/usr/bin/env python3
"""
插入测试数据到 test_db_sync 数据库
"""

import pymysql
import random
from datetime import datetime, timedelta
import json

config = {
    "host": "127.0.0.1",
    "port": 3306,
    "user": "root",
    "password": "",
    "database": "test_db_sync",
    "charset": "utf8mb4",
}


# 测试数据生成器
class DataGenerator:
    def __init__(self):
        self.dept_names = [
            "研发部",
            "产品部",
            "设计部",
            "测试部",
            "运维部",
            "市场部",
            "销售部",
            "人事部",
            "财务部",
            "行政部",
        ]
        self.locations = [
            "北京",
            "上海",
            "深圳",
            "杭州",
            "成都",
            "广州",
            "西安",
            "武汉",
        ]
        self.first_names = [
            "伟",
            "芳",
            "娜",
            "敏",
            "静",
            "强",
            "磊",
            "洋",
            "勇",
            "军",
            "杰",
            "娟",
            "艳",
            "涛",
            "明",
        ]
        self.last_names = [
            "王",
            "李",
            "张",
            "刘",
            "陈",
            "杨",
            "黄",
            "赵",
            "周",
            "吴",
            "徐",
            "孙",
            "马",
            "朱",
            "胡",
        ]
        self.positions = [
            "工程师",
            "经理",
            "主管",
            "总监",
            "专员",
            "助理",
            "架构师",
            "分析师",
            "顾问",
        ]
        self.project_names = [
            "电商平台重构",
            "移动端APP开发",
            "数据中台建设",
            "AI智能客服",
            "云原生迁移",
            "支付系统升级",
            "用户画像系统",
            "推荐算法优化",
            "大数据平台建设",
            "DevOps体系建设",
            "安全防护升级",
            "监控系统完善",
            "自动化测试平台",
            "文档管理系统",
            "内部工具开发",
        ]
        self.proj_descriptions = [
            "提升系统性能和用户体验的综合性项目",
            "基于最新技术栈的全新产品开发",
            "整合各业务线数据，提供统一数据服务",
            "利用人工智能技术提升服务效率",
            "将传统架构迁移到云原生架构",
        ]

    def random_name(self):
        return random.choice(self.last_names) + random.choice(self.first_names)

    def random_date(self, start_year=2015, end_year=2024):
        start = datetime(start_year, 1, 1)
        end = datetime(end_year, 12, 31)
        days = (end - start).days
        return start + timedelta(days=random.randint(0, days))

    def random_email(self, name):
        domains = ["company.com", "test.com", "example.com", "demo.org"]
        return f"{name.lower()}{random.randint(1, 999)}@{random.choice(domains)}"


def insert_departments(cursor):
    """插入部门数据"""
    gen = DataGenerator()
    departments = []

    for i, dept_name in enumerate(gen.dept_names, 1):
        dept = {
            "dept_code": f"DEPT{i:03d}",
            "dept_name": dept_name,
            "location": random.choice(gen.locations),
            "budget": random.randint(500000, 5000000),
            "established_date": gen.random_date(2010, 2020).strftime("%Y-%m-%d"),
            "is_active": random.random() > 0.1,  # 90%激活
        }
        departments.append(dept)

    sql = """
        INSERT INTO departments (dept_code, dept_name, location, budget, established_date, is_active)
        VALUES (%(dept_code)s, %(dept_name)s, %(location)s, %(budget)s, %(established_date)s, %(is_active)s)
    """
    cursor.executemany(sql, departments)
    return len(departments)


def insert_employees(cursor, dept_ids):
    """插入员工数据"""
    gen = DataGenerator()
    employees = []

    for i in range(1, 201):  # 200个员工
        name = gen.random_name()
        birth = gen.random_date(1975, 2000)
        hire = gen.random_date(2015, 2024)

        emp = {
            "emp_no": f"E{2024}{i:05d}",
            "name": name,
            "email": gen.random_email(name),
            "phone": f"1{random.choice(['3', '4', '5', '7', '8', '9'])}{random.randint(100000000, 999999999)}",
            "gender": random.choice(["M", "F", "O"]),
            "birth_date": birth.strftime("%Y-%m-%d"),
            "hire_date": hire.strftime("%Y-%m-%d"),
            "department_id": random.choice(dept_ids)
            if random.random() > 0.05
            else None,
            "position": random.choice(gen.positions),
            "salary_base": random.randint(8000, 80000),
            "status": random.choice(
                ["active", "active", "active", "inactive", "on_leave", "terminated"]
            ),
            "avatar_url": f"https://avatars.company.com/{i}.jpg"
            if random.random() > 0.3
            else None,
            "metadata": json.dumps(
                {
                    "education": random.choice(["本科", "硕士", "博士", "大专"]),
                    "major": random.choice(
                        ["计算机", "软件工程", "电子信息", "工商管理", "经济学"]
                    ),
                    "entry_channel": random.choice(["校招", "社招", "内推", "猎头"]),
                    "emergency_contact": gen.random_name(),
                    "emergency_phone": f"1{random.choice(['3', '4', '5', '7', '8', '9'])}{random.randint(100000000, 999999999)}",
                }
            )
            if random.random() > 0.2
            else None,
        }
        employees.append(emp)

    sql = """
        INSERT INTO employees 
        (emp_no, name, email, phone, gender, birth_date, hire_date, department_id, position, salary_base, status, avatar_url, metadata)
        VALUES 
        (%(emp_no)s, %(name)s, %(email)s, %(phone)s, %(gender)s, %(birth_date)s, %(hire_date)s, 
         %(department_id)s, %(position)s, %(salary_base)s, %(status)s, %(avatar_url)s, %(metadata)s)
    """
    cursor.executemany(sql, employees)
    return len(employees)


def insert_projects(cursor, emp_ids):
    """插入项目数据"""
    gen = DataGenerator()
    projects = []
    statuses = ["planning", "ongoing", "completed", "cancelled", "on_hold"]
    priorities = ["low", "medium", "high", "urgent"]

    for i in range(1, 51):  # 50个项目
        start = gen.random_date(2020, 2024)
        duration = random.randint(30, 730)  # 1个月到2年
        end = start + timedelta(days=duration)
        budget = random.randint(100000, 10000000)
        actual = budget * random.uniform(0.5, 1.5)
        status = random.choice(statuses)
        progress = 0
        if status == "completed":
            progress = 100
        elif status == "ongoing":
            progress = random.randint(10, 90)
        elif status == "cancelled":
            progress = random.randint(10, 80)

        proj = {
            "proj_code": f"PROJ{2024}{i:04d}",
            "proj_name": random.choice(gen.project_names) + f"-{i}",
            "description": random.choice(gen.proj_descriptions),
            "manager_id": random.choice(emp_ids) if random.random() > 0.1 else None,
            "status": status,
            "priority": random.choice(priorities),
            "start_date": start.strftime("%Y-%m-%d"),
            "end_date": end.strftime("%Y-%m-%d")
            if status in ["completed", "cancelled"]
            else None,
            "budget": budget,
            "actual_cost": actual if status in ["completed", "ongoing"] else None,
            "progress": progress,
            "tags": ",".join(
                random.sample(
                    ["重要", "紧急", "长期", "短期", "创新", "维护", "核心", "边缘"],
                    k=random.randint(1, 4),
                )
            ),
        }
        projects.append(proj)

    sql = """
        INSERT INTO projects 
        (proj_code, proj_name, description, manager_id, status, priority, start_date, end_date, budget, actual_cost, progress, tags)
        VALUES 
        (%(proj_code)s, %(proj_name)s, %(description)s, %(manager_id)s, %(status)s, %(priority)s, 
         %(start_date)s, %(end_date)s, %(budget)s, %(actual_cost)s, %(progress)s, %(tags)s)
    """
    cursor.executemany(sql, projects)
    return len(projects)


def insert_employee_projects(cursor, emp_ids, proj_ids):
    """插入员工项目关联数据"""
    relations = []
    roles = [
        "开发工程师",
        "产品经理",
        "测试工程师",
        "UI设计师",
        "项目经理",
        "技术负责人",
        "业务分析师",
    ]

    # 每个项目分配3-10个员工
    for proj_id in proj_ids:
        num_employees = random.randint(3, 10)
        selected_emps = random.sample(emp_ids, min(num_employees, len(emp_ids)))

        for emp_id in selected_emps:
            join_date = datetime(2020, 1, 1) + timedelta(days=random.randint(0, 1500))
            leave_date = None
            if random.random() > 0.7:  # 30%已离开项目
                leave_date = join_date + timedelta(days=random.randint(30, 500))

            rel = {
                "employee_id": emp_id,
                "project_id": proj_id,
                "role": random.choice(roles),
                "join_date": join_date.strftime("%Y-%m-%d"),
                "leave_date": leave_date.strftime("%Y-%m-%d") if leave_date else None,
                "allocation": random.choice([25, 50, 75, 100]),
                "is_primary": random.random() > 0.8,
            }
            relations.append(rel)

    sql = """
        INSERT INTO employee_projects 
        (employee_id, project_id, role, join_date, leave_date, allocation, is_primary)
        VALUES 
        (%(employee_id)s, %(project_id)s, %(role)s, %(join_date)s, %(leave_date)s, %(allocation)s, %(is_primary)s)
    """
    cursor.executemany(sql, relations)
    return len(relations)


def insert_salary_records(cursor, emp_ids):
    """插入薪资记录数据"""
    records = []

    # 为每个员工生成最近24个月的薪资记录
    for emp_id in emp_ids:
        base = random.randint(8000, 80000)
        today = datetime.now()
        periods = []
        for month_offset in range(24):
            year = today.year
            month = today.month - month_offset
            while month <= 0:
                month += 12
                year -= 1
            periods.append(f"{year:04d}-{month:02d}")
        for period in periods:
            period_date = datetime.strptime(period, "%Y-%m")

            bonus = base * random.uniform(0, 0.3) if random.random() > 0.5 else 0
            overtime = base * random.uniform(0, 0.1) if random.random() > 0.7 else 0
            deduction = base * random.uniform(0, 0.05) if random.random() > 0.8 else 0
            tax = base * random.uniform(0.05, 0.15)
            insurance = base * random.uniform(0.1, 0.15)
            net = base + bonus + overtime - deduction - tax - insurance

            record = {
                "employee_id": emp_id,
                "pay_period": period,
                "base_salary": base,
                "bonus": round(bonus, 2),
                "overtime_pay": round(overtime, 2),
                "deduction": round(deduction, 2),
                "tax": round(tax, 2),
                "insurance": round(insurance, 2),
                "net_pay": round(net, 2),
                "pay_date": (
                    period_date + timedelta(days=random.randint(5, 10))
                ).strftime("%Y-%m-%d"),
                "status": random.choice(["paid", "paid", "paid", "confirmed", "draft"]),
                "remarks": random.choice(
                    ["正常发放", "季度奖金", "年终奖", "调薪补差", None, None, None]
                ),
            }
            records.append(record)

    # 分批插入，避免SQL过长
    batch_size = 1000
    sql = """
        INSERT INTO salary_records 
        (employee_id, pay_period, base_salary, bonus, overtime_pay, deduction, tax, insurance, net_pay, pay_date, status, remarks)
        VALUES 
        (%(employee_id)s, %(pay_period)s, %(base_salary)s, %(bonus)s, %(overtime_pay)s, %(deduction)s, 
         %(tax)s, %(insurance)s, %(net_pay)s, %(pay_date)s, %(status)s, %(remarks)s)
    """

    for i in range(0, len(records), batch_size):
        batch = records[i : i + batch_size]
        cursor.executemany(sql, batch)

    return len(records)


def insert_operation_logs(cursor):
    """插入大量操作日志数据（用于测试大数据量导入导出）"""
    logs = []
    tables = ["employees", "departments", "projects", "salary_records"]
    ops = ["INSERT", "UPDATE", "DELETE"]

    # 生成10000条日志
    for i in range(10000):
        table = random.choice(tables)
        op = random.choice(ops)
        old_vals = None
        new_vals = None

        if op in ["UPDATE", "DELETE"]:
            old_vals = json.dumps(
                {"field1": "old_value", "field2": random.randint(1, 100)}
            )
        if op in ["INSERT", "UPDATE"]:
            new_vals = json.dumps(
                {"field1": "new_value", "field2": random.randint(1, 100)}
            )

        log = {
            "table_name": table,
            "operation_type": op,
            "record_id": str(random.randint(1, 10000)),
            "old_values": old_vals,
            "new_values": new_vals,
            "operator_id": random.randint(1, 200),
            "operator_name": DataGenerator().random_name(),
            "ip_address": f"192.168.{random.randint(0, 255)}.{random.randint(1, 254)}",
            "user_agent": random.choice(
                [
                    "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
                    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36",
                    "Mozilla/5.0 (iPhone; CPU iPhone OS 14_0 like Mac OS X)",
                ]
            ),
        }
        logs.append(log)

    # 分批插入
    batch_size = 2000
    sql = """
        INSERT INTO operation_logs 
        (table_name, operation_type, record_id, old_values, new_values, operator_id, operator_name, ip_address, user_agent)
        VALUES 
        (%(table_name)s, %(operation_type)s, %(record_id)s, %(old_values)s, %(new_values)s, 
         %(operator_id)s, %(operator_name)s, %(ip_address)s, %(user_agent)s)
    """

    for i in range(0, len(logs), batch_size):
        batch = logs[i : i + batch_size]
        cursor.executemany(sql, batch)

    return len(logs)


def main():
    conn = pymysql.connect(**config)
    cursor = conn.cursor()

    try:
        tables = [
            "operation_logs",
            "salary_records",
            "employee_projects",
            "projects",
            "employees",
            "departments",
        ]
        for table in tables:
            cursor.execute(f"DELETE FROM {table}")
        conn.commit()

        dept_count = insert_departments(cursor)
        conn.commit()
        print(f"✓ 插入 {dept_count} 个部门")

        # 获取部门ID列表
        cursor.execute("SELECT id FROM departments")
        dept_ids = [row[0] for row in cursor.fetchall()]

        # 2. 插入员工
        emp_count = insert_employees(cursor, dept_ids)
        conn.commit()
        print(f"✓ 插入 {emp_count} 个员工")

        # 获取员工ID列表
        cursor.execute("SELECT id FROM employees")
        emp_ids = [row[0] for row in cursor.fetchall()]

        # 3. 插入项目
        proj_count = insert_projects(cursor, emp_ids)
        conn.commit()
        print(f"✓ 插入 {proj_count} 个项目")

        # 获取项目ID列表
        cursor.execute("SELECT id FROM projects")
        proj_ids = [row[0] for row in cursor.fetchall()]

        # 4. 插入员工项目关联
        rel_count = insert_employee_projects(cursor, emp_ids, proj_ids)
        conn.commit()
        print(f"✓ 插入 {rel_count} 条员工项目关联")

        # 5. 插入薪资记录
        salary_count = insert_salary_records(cursor, emp_ids)
        conn.commit()
        print(f"✓ 插入 {salary_count} 条薪资记录")

        # 6. 插入操作日志
        log_count = insert_operation_logs(cursor)
        conn.commit()
        print(f"✓ 插入 {log_count} 条操作日志")

        print("\n" + "=" * 50)
        print("测试数据插入完成！")
        print("=" * 50)
        print(f"\n数据库: test_db_sync")
        print(f"数据表: 6个")
        print(
            f"总记录数: {dept_count + emp_count + proj_count + rel_count + salary_count + log_count:,} 条"
        )
        print("\n数据表统计:")
        cursor.execute("SHOW TABLES")
        for (table,) in cursor.fetchall():
            cursor.execute(f"SELECT COUNT(*) FROM {table}")
            count = cursor.fetchone()[0]
            print(f"  - {table}: {count:,} 条")

    except Exception as e:
        conn.rollback()
        print(f"错误: {e}")
        raise
    finally:
        cursor.close()
        conn.close()


if __name__ == "__main__":
    main()
