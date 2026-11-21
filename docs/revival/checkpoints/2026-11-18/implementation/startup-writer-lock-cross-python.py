import os, sys, time
from pathlib import Path
import vyatta_password_persistence as persistence
base, ident = Path(sys.argv[1]), sys.argv[2]
with persistence.SavedPasswordUpdate('tester', filename=base/'etc/config/config.boot', lock_dir=base/'lock'):
    (base/(ident+'.entered')).write_text('1')
    while not (base/(ident+'.release')).exists():
        time.sleep(.02)
