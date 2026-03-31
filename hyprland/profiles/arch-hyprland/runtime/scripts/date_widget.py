import datetime
import os
import time

while True:
    now = datetime.datetime.now()
    os.system('printf "\\033[2J\\033[H"')
    print('\n\n')
    print(f'                {now:%H:%M}')
    print('\n')
    print(f'             {now:%A}')
    print(f'             {now:%Y-%m-%d}')
    time.sleep(1)
