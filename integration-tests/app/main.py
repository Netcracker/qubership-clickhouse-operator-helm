import os
from flask import Flask

app = Flask(__name__)


# index page  (report.html)
@app.route('/')
def index():
    return main_page()


# so, here <name> is actual file name,after robot run
# it produces log, report html files
@app.route('/<name>')
def main_page(name=None):
    if not name:
        name = 'report.html'

    file_name = '/app/{}'.format(name)

    if os.path.exists(file_name):
        data = open(file_name, 'r').read()
    else:
        data = 'Can not find report or log file after robot run.'

    return data


if __name__ == '__main__':
    app.run(debug=True, host='0.0.0.0')
