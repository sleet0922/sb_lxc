import os, sys
sys.path.insert(0, '/app')
import appmod  # 来自 TEMP builder 块的产物

from flask import Flask, jsonify
app = Flask(__name__)

@app.route('/')
def index():
    return jsonify({
        'app': 'fedora-python',
        'module': 'appmod',
        'temp_block_output': appmod.hello(),
        'flask_app': os.environ.get('FLASK_APP', ''),
    })

@app.route('/health')
def health():
    return jsonify({'status': 'ok', 'distro': 'fedora/44'})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)
