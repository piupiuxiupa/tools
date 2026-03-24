#!/usr/bin/env python3
"""
创建测试数据库和表结构，用于测试数据导入导出功能
"""

import pymysql

# 连接配置
config = {
    "host": "127.0.0.1",
    "port": 3306,
    "user": "root",
    "password": "",
    "charset": "utf8mb4",
}


def create_database_and_tables():
    conn = pymysql.connect(**config)
    cursor = conn.cursor()

    # 创建数据库
    cursor.execute("DROP DATABASE IF EXISTS test_db_sync")
    cursor.execute(
        "CREATE DATABASE test_db_sync CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci"
    )
    cursor.execute("USE test_db_sync")

    # 1. 部门表
    cursor.execute("""
        CREATE TABLE departments (
            id INT PRIMARY KEY AUTO_INCREMENT,
            dept_code VARCHAR(20) NOT NULL UNIQUE COMMENT '部门编码',
            dept_name VARCHAR(100) NOT NULL COMMENT '部门名称',
            location VARCHAR(100) COMMENT '办公地点',
            budget DECIMAL(15, 2) COMMENT '年度预算',
            established_date DATE COMMENT '成立日期',
            is_active BOOLEAN DEFAULT TRUE COMMENT '是否启用',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            INDEX idx_location (location),
            INDEX idx_active (is_active)
        ) ENGINE=InnoDB COMMENT='部门信息表'
    """)

    # 2. 员工表
    cursor.execute("""
        CREATE TABLE employees (
            id INT PRIMARY KEY AUTO_INCREMENT,
            emp_no VARCHAR(20) NOT NULL UNIQUE COMMENT '员工编号',
            name VARCHAR(50) NOT NULL COMMENT '姓名',
            email VARCHAR(100) UNIQUE COMMENT '邮箱',
            phone VARCHAR(20) COMMENT '电话',
            gender ENUM('M', 'F', 'O') COMMENT '性别',
            birth_date DATE COMMENT '出生日期',
            hire_date DATE NOT NULL COMMENT '入职日期',
            department_id INT COMMENT '所属部门',
            position VARCHAR(50) COMMENT '职位',
            salary_base DECIMAL(10, 2) COMMENT '基本工资',
            status ENUM('active', 'inactive', 'on_leave', 'terminated') DEFAULT 'active' COMMENT '状态',
            avatar_url VARCHAR(255) COMMENT '头像URL',
            metadata JSON COMMENT '扩展信息',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            FOREIGN KEY (department_id) REFERENCES departments(id) ON DELETE SET NULL,
            INDEX idx_dept (department_id),
            INDEX idx_status (status),
            INDEX idx_hire_date (hire_date)
        ) ENGINE=InnoDB COMMENT='员工信息表'
    """)

    # 3. 项目表
    cursor.execute("""
        CREATE TABLE projects (
            id INT PRIMARY KEY AUTO_INCREMENT,
            proj_code VARCHAR(30) NOT NULL UNIQUE COMMENT '项目编码',
            proj_name VARCHAR(200) NOT NULL COMMENT '项目名称',
            description TEXT COMMENT '项目描述',
            manager_id INT COMMENT '项目经理',
            status ENUM('planning', 'ongoing', 'completed', 'cancelled', 'on_hold') DEFAULT 'planning' COMMENT '状态',
            priority ENUM('low', 'medium', 'high', 'urgent') DEFAULT 'medium' COMMENT '优先级',
            start_date DATE COMMENT '开始日期',
            end_date DATE COMMENT '结束日期',
            budget DECIMAL(15, 2) COMMENT '预算',
            actual_cost DECIMAL(15, 2) COMMENT '实际成本',
            progress TINYINT UNSIGNED DEFAULT 0 COMMENT '进度百分比',
            tags VARCHAR(255) COMMENT '标签',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            FOREIGN KEY (manager_id) REFERENCES employees(id) ON DELETE SET NULL,
            INDEX idx_status (status),
            INDEX idx_priority (priority),
            INDEX idx_dates (start_date, end_date)
        ) ENGINE=InnoDB COMMENT='项目信息表'
    """)

    # 4. 员工项目关联表（多对多关系）
    cursor.execute("""
        CREATE TABLE employee_projects (
            id INT PRIMARY KEY AUTO_INCREMENT,
            employee_id INT NOT NULL COMMENT '员工ID',
            project_id INT NOT NULL COMMENT '项目ID',
            role VARCHAR(50) COMMENT '担任角色',
            join_date DATE NOT NULL COMMENT '加入日期',
            leave_date DATE COMMENT '离开日期',
            allocation DECIMAL(5, 2) DEFAULT 100.00 COMMENT '投入比例(%)',
            is_primary BOOLEAN DEFAULT FALSE COMMENT '是否主负责',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            FOREIGN KEY (employee_id) REFERENCES employees(id) ON DELETE CASCADE,
            FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE CASCADE,
            UNIQUE KEY uk_emp_proj (employee_id, project_id),
            INDEX idx_project (project_id)
        ) ENGINE=InnoDB COMMENT='员工项目关联表'
    """)

    # 5. 薪资记录表
    cursor.execute("""
        CREATE TABLE salary_records (
            id INT PRIMARY KEY AUTO_INCREMENT,
            employee_id INT NOT NULL COMMENT '员工ID',
            pay_period VARCHAR(20) NOT NULL COMMENT '薪资周期(YYYY-MM)',
            base_salary DECIMAL(10, 2) NOT NULL COMMENT '基本工资',
            bonus DECIMAL(10, 2) DEFAULT 0 COMMENT '奖金',
            overtime_pay DECIMAL(10, 2) DEFAULT 0 COMMENT '加班费',
            deduction DECIMAL(10, 2) DEFAULT 0 COMMENT '扣款',
            tax DECIMAL(10, 2) DEFAULT 0 COMMENT '税额',
            insurance DECIMAL(10, 2) DEFAULT 0 COMMENT '社保',
            net_pay DECIMAL(10, 2) COMMENT '实发工资',
            pay_date DATE COMMENT '发放日期',
            status ENUM('draft', 'confirmed', 'paid') DEFAULT 'draft' COMMENT '状态',
            remarks VARCHAR(255) COMMENT '备注',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
            FOREIGN KEY (employee_id) REFERENCES employees(id) ON DELETE CASCADE,
            UNIQUE KEY uk_emp_period (employee_id, pay_period),
            INDEX idx_pay_period (pay_period),
            INDEX idx_status (status)
        ) ENGINE=InnoDB COMMENT='薪资记录表'
    """)

    # 6. 操作日志表（大量数据测试用）
    cursor.execute("""
        CREATE TABLE operation_logs (
            id BIGINT PRIMARY KEY AUTO_INCREMENT,
            table_name VARCHAR(50) NOT NULL COMMENT '表名',
            operation_type ENUM('INSERT', 'UPDATE', 'DELETE') NOT NULL COMMENT '操作类型',
            record_id VARCHAR(50) NOT NULL COMMENT '记录ID',
            old_values JSON COMMENT '旧值',
            new_values JSON COMMENT '新值',
            operator_id INT COMMENT '操作人',
            operator_name VARCHAR(50) COMMENT '操作人姓名',
            ip_address VARCHAR(50) COMMENT 'IP地址',
            user_agent VARCHAR(500) COMMENT 'UserAgent',
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            INDEX idx_table_op (table_name, operation_type),
            INDEX idx_created (created_at),
            INDEX idx_operator (operator_id)
        ) ENGINE=InnoDB COMMENT='操作日志表'
    """)

    conn.commit()
    print("✓ 数据库和表结构创建完成")

    cursor.close()
    conn.close()


if __name__ == "__main__":
    create_database_and_tables()
