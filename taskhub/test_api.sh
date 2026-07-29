#!/bin/bash
set -e
BASE=http://taskhub.test:8080

echo '=== 1. 创建任务: 测试多阶段构建 ==='
R1=$(curl -s -X POST $BASE/api/v1/tasks -H 'Content-Type: application/json' -d '{"title":"测试多阶段构建"}')
echo "$R1"
TASK_ID=$(echo "$R1" | grep -o '"id":[0-9]*' | head -1 | cut -d: -f2)
echo "任务 ID: $TASK_ID"

echo '=== 2. 创建任务: 验证 MariaDB 持久化 ==='
curl -s -X POST $BASE/api/v1/tasks -H 'Content-Type: application/json' -d '{"title":"验证 MariaDB 持久化"}'
echo

echo '=== 3. 列出所有任务 ==='
curl -s $BASE/api/v1/tasks
echo

echo '=== 4. 查看统计 (应走 DB, source=db) ==='
curl -s $BASE/stats
echo

echo '=== 5. 查看统计 (应走 Redis 缓存, source=cache) ==='
curl -s $BASE/stats
echo

echo "=== 6. 标记任务 $TASK_ID 为完成 ==="
curl -s -X PUT $BASE/api/v1/tasks/$TASK_ID/done
echo

echo '=== 7. 查看统计 (缓存已失效, 应走 DB) ==='
curl -s $BASE/stats
echo

echo "=== 8. 删除任务 $TASK_ID ==="
curl -s -X DELETE $BASE/api/v1/tasks/$TASK_ID
echo

echo '=== 9. 最终任务列表 ==='
curl -s $BASE/api/v1/tasks
echo

echo '=== 10. 最终统计 ==='
curl -s $BASE/stats
echo

echo '=== 11. 验证 Redis 缓存工作 (连续两次 stats) ==='
curl -s $BASE/stats
echo
curl -s $BASE/stats
echo
