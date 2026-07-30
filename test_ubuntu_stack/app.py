import os, sys
import pymysql
import redis
from flask import Flask, jsonify

app = Flask(__name__)

def db_conn():
    return pymysql.connect(
        host=os.environ.get('DB_HOST', '127.0.0.1'),
        user=os.environ.get('DB_USER', 'appuser'),
        password=os.environ.get('DB_PASSWORD', ''),
        database=os.environ.get('DB_NAME', 'appdb'),
        autocommit=True,
    )

def redis_conn():
    return redis.Redis(
        host=os.environ.get('REDIS_HOST', '127.0.0.1'),
        port=int(os.environ.get('REDIS_PORT', '6379')),
        decode_responses=True,
    )

@app.route('/health')
def health():
    try:
        db = db_conn()
        with db.cursor() as cur:
            cur.execute('SELECT 1')
        db.close()
        r = redis_conn()
        r.ping()
        return jsonify({'status': 'ok', 'services': ['mariadb', 'redis', 'flask']})
    except Exception as e:
        return jsonify({'status': 'error', 'error': str(e)}), 500

@app.route('/api/count', methods=['POST'])
def increment():
    db = db_conn()
    with db.cursor() as cur:
        cur.execute('UPDATE counter SET count = count + 1 WHERE id = 1')
        cur.execute('SELECT count FROM counter WHERE id = 1')
        count = cur.fetchone()[0]
    db.close()
    r = redis_conn()
    r.incr('total_requests')
    return jsonify({'count': count, 'redis_total': r.get('total_requests')})

@app.route('/api/count', methods=['GET'])
def get_count():
    db = db_conn()
    with db.cursor() as cur:
        cur.execute('SELECT count FROM counter WHERE id = 1')
        count = cur.fetchone()[0]
    db.close()
    return jsonify({'count': count})

@app.route('/')
def index():
    return jsonify({
        'app': 'ubuntu-stack',
        'services': ['mariadb', 'redis', 'flask'],
        'endpoints': ['/health', '/api/count (GET/POST)'],
    })

if __name__ == '__main__':
    port = int(os.environ.get('LISTEN_PORT', '8080'))
    app.run(host='0.0.0.0', port=port)
