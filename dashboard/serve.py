from waitress import serve

from .app import HOST, PORT, app

serve(app, host=HOST, port=PORT)
