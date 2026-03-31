import os
import subprocess
import time

BARS = ['▁','▂','▃','▄','▅','▆','▇','█']
frame = 0


def query(*parts):
    try:
        return subprocess.check_output(parts, stderr=subprocess.DEVNULL, text=True).strip()
    except Exception:
        return ''


while True:
    status = query('playerctl', 'status')
    title = query('playerctl', 'metadata', 'title') or 'Unknown title'
    artist = query('playerctl', 'metadata', 'artist') or 'Unknown artist'
    os.system('printf "\\033[2J\\033[H"')
    print('\n\n')
    if status:
        print('  NOW PLAYING\n')
        print(f'  {title}')
        print(f'  {artist}\n')
        if status == 'Playing':
            line = ' '.join(BARS[(frame + i * 3) % len(BARS)] for i in range(18))
            print(f'  {line}\n')
            print('  visualizer only shows while media is active')
            frame = (frame + 1) % len(BARS)
        else:
            print('  media detected, paused right now\n')
            print('  visualizer only shows while media is active')
    else:
        print('  NO MEDIA\n')
        print('  open spotify, firefox, mpv, or any MPRIS player')
        print('  visualizer stays hidden until something is playing')
    time.sleep(1)
