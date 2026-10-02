"""Deterministic, offline bilingual guitar catalog compiler (Python stdlib only)."""
import argparse
import hashlib
import itertools
import json
import re
import uuid
from collections import Counter
from pathlib import Path

NAMESPACE = uuid.UUID('4ac75155-7804-5527-a6ba-01b73c0e3e1a')
TUNING = [40, 45, 50, 55, 59, 64]
MAX_FRET = 12
ROOTS = ['C', 'Db', 'D', 'Eb', 'E', 'F', 'Gb', 'G', 'Ab', 'A', 'Bb', 'B']
LETTERS = 'CDEFGAB'
NATURAL = [0, 2, 4, 5, 7, 9, 11]
PT = dict(zip(LETTERS, ['Dó', 'Ré', 'Mi', 'Fá', 'Sol', 'Lá', 'Si']))
INTERVALS = 'R b2 2 #2 b3 3 4 #4 b5 5 #5 b6 6 bb7 b7 7 b9 9 #9 11 #11 b13 13'.split()
CHROMATIC = 'R b2 2 b3 3 4 b5 5 b6 6 b7 7'.split()
# key: English, Brazilian Portuguese, degree spelling, tier, diatonic mode or None
FORMULAS = {
    'major': ('Major scale', 'Escala maior', 'R 2 3 4 5 6 7', 'A', 'major'),
    'natural-minor': ('Natural minor scale', 'Escala menor natural', 'R 2 b3 4 5 b6 b7', 'A', 'minor'),
    'major-pentatonic': ('Major pentatonic', 'Pentatônica maior', 'R 2 3 5 6', 'A', 'major'),
    'minor-pentatonic': ('Minor pentatonic', 'Pentatônica menor', 'R b3 4 5 b7', 'A', 'minor'),
    'minor-blues': ('Minor blues scale', 'Escala blues menor', 'R b3 4 b5 5 b7', 'A', 'minor'),
    'major-blues': ('Major blues scale', 'Escala blues maior', 'R 2 b3 3 5 6', 'A', 'major'),
    'dorian': ('Dorian mode', 'Modo dórico', 'R 2 b3 4 5 6 b7', 'B', 'dorian'),
    'phrygian': ('Phrygian mode', 'Modo frígio', 'R b2 b3 4 5 b6 b7', 'B', 'phrygian'),
    'lydian': ('Lydian mode', 'Modo lídio', 'R 2 3 #4 5 6 7', 'B', 'lydian'),
    'mixolydian': ('Mixolydian mode', 'Modo mixolídio', 'R 2 3 4 5 6 b7', 'B', 'mixolydian'),
    'locrian': ('Locrian mode', 'Modo lócrio', 'R b2 b3 4 b5 b6 b7', 'B', 'locrian'),
    'harmonic-minor': ('Harmonic minor scale', 'Escala menor harmônica', 'R 2 b3 4 5 b6 7', 'B', 'minor'),
    'melodic-minor': ('Jazz melodic minor scale', 'Escala menor melódica do jazz', 'R 2 b3 4 5 6 7', 'B', 'minor'),
    'whole-tone': ('Whole-tone scale', 'Escala de tons inteiros', 'R 2 3 #4 #5 b7', 'C', None),
    'half-whole': ('Half-whole diminished scale', 'Escala diminuta semitom–tom', 'R b2 b3 3 b5 5 6 b7', 'C', None),
    'whole-half': ('Whole-half diminished scale', 'Escala diminuta tom–semitom', 'R 2 b3 4 b5 b6 6 7', 'C', None),
    'major-triad': ('Major triad', 'Tríade maior', 'R 3 5', 'A', 'major'),
    'minor-triad': ('Minor triad', 'Tríade menor', 'R b3 5', 'A', 'minor'),
    'diminished-triad': ('Diminished triad', 'Tríade diminuta', 'R b3 b5', 'A', None),
    'augmented-triad': ('Augmented triad', 'Tríade aumentada', 'R 3 #5', 'A', None),
    'maj7': ('Major seventh chord', 'Acorde maior com sétima maior', 'R 3 5 7', 'A', 'major'),
    'dom7': ('Dominant seventh chord', 'Acorde dominante com sétima', 'R 3 5 b7', 'A', None),
    'min7': ('Minor seventh chord', 'Acorde menor com sétima', 'R b3 5 b7', 'A', 'minor'),
    'half-dim7': ('Half-diminished seventh chord', 'Acorde meio-diminuto', 'R b3 b5 b7', 'A', None),
    'dim7': ('Diminished seventh chord', 'Acorde de sétima diminuta', 'R b3 b5 bb7', 'A', None),
    'maj9': ('Major ninth arpeggio', 'Arpejo maior com nona e sétima maior', 'R 3 5 7 9', 'B', 'major'),
    'dom9': ('Dominant ninth arpeggio', 'Arpejo dominante com nona', 'R 3 5 b7 9', 'B', None),
    'min9': ('Minor ninth arpeggio', 'Arpejo menor com nona', 'R b3 5 b7 9', 'B', 'minor'),
    'maj11': ('Major eleventh arpeggio', 'Arpejo maior com décima primeira e sétima maior', 'R 3 5 7 9 11', 'B', 'major'),
    'dom11': ('Dominant eleventh arpeggio', 'Arpejo dominante com décima primeira', 'R 3 5 b7 9 11', 'B', None),
    'min11': ('Minor eleventh arpeggio', 'Arpejo menor com décima primeira', 'R b3 5 b7 9 11', 'B', 'minor'),
    'maj13': ('Major thirteenth arpeggio', 'Arpejo maior com décima terceira e sétima maior', 'R 3 5 7 9 11 13', 'B', 'major'),
    'dom13': ('Dominant thirteenth arpeggio', 'Arpejo dominante com décima terceira', 'R 3 5 b7 9 11 13', 'B', None),
    'min13': ('Minor thirteenth arpeggio', 'Arpejo menor com décima terceira', 'R b3 5 b7 9 11 13', 'B', 'minor'),
}
TRIADS = ['major-triad', 'minor-triad', 'diminished-triad', 'augmented-triad']
SEVENTHS = ['maj7', 'dom7', 'min7', 'half-dim7', 'dim7']
PENT_BOXES = [
    [(5,8),(5,7),(5,7),(5,7),(5,8),(5,8)],
    [(8,10),(7,10),(7,10),(7,9),(8,10),(8,10)],
    [(10,12),(10,12),(10,12),(9,12),(10,13),(10,12)],
    [(12,15),(12,15),(12,14),(12,14),(13,15),(12,15)],
    [(15,17),(15,17),(14,17),(14,17),(15,17),(15,17)],
]
CAGED = {
    'C': ('C', [(5,3),(4,2),(3,0),(2,1),(1,0)]),
    'A': ('A', [(5,0),(4,2),(3,2),(2,2),(1,0)]),
    'G': ('G', [(6,3),(5,2),(4,0),(3,0),(2,0),(1,3)]),
    'E': ('E', [(6,0),(5,2),(4,2),(3,1),(2,0),(1,0)]),
    'D': ('D', [(4,0),(3,2),(2,3),(1,2)]),
}


def stable_id(key):
    return str(uuid.uuid5(NAMESPACE, key))


SYSTEM_CATALOG_USER_ID = stable_id('user/system-catalog')
SYSTEM_CATALOG_CLERK_USER_ID = 'system:catalog'
SYSTEM_CATALOG_DISPLAY_NAME = 'MotifPath Catalog'
INSTRUMENTS = {
    'guitar': {'en': 'Guitar', 'pt_BR': 'Violão'},
    'electric-guitar': {'en': 'Electric guitar', 'pt_BR': 'Guitarra elétrica'},
}


def semitones(interval):
    if interval == 'R': return 0
    match = re.fullmatch(r'(bb|b|##|#)?(\d+)', interval)
    if not match: raise ValueError(interval)
    accidental, degree = match.groups(); degree = int(degree)
    return NATURAL[(degree-1) % 7] + 12*((degree-1)//7) + (accidental or '').count('#') - (accidental or '').count('b')


def pitch_class(note):
    if not re.fullmatch(r'[A-G](bb|b|##|#)?', note): raise ValueError(note)
    return (NATURAL[LETTERS.index(note[0])] + note.count('#') - note.count('b')) % 12


def spell(root, interval):
    degree = 1 if interval == 'R' else int(re.search(r'\d+', interval).group())
    letter = LETTERS[(LETTERS.index(root[0]) + degree - 1) % 7]
    target = (pitch_class(root) + semitones(interval)) % 12
    diff = (target - NATURAL[LETTERS.index(letter)] + 6) % 12 - 6
    if abs(diff) > 2: raise ValueError(f'Unsupported spelling: {root}/{interval}')
    return letter + ('#'*diff if diff > 0 else 'b'*(-diff))


def root_pt(root):
    return PT[root[0]] + (' sustenido'*root.count('#')) + (' bemol'*root.count('b'))


def cells(root, intervals, lo=0, hi=MAX_FRET):
    by_pitch = {(pitch_class(root)+semitones(i)) % 12: i for i in intervals}
    return [(s, f, by_pitch[(TUNING[6-s]+f) % 12]) for s in range(6,0,-1) for f in range(lo,hi+1) if (TUNING[6-s]+f) % 12 in by_pitch]


def tagged(root, intervals, coordinates):
    by_pitch = {(pitch_class(root)+semitones(i)) % 12: i for i in intervals}
    return [(s,f,by_pitch[(TUNING[6-s]+f) % 12]) for s,f in coordinates]


def entry(key, root, intervals, coordinates, en, pt, family, tier, formula='', mode=None, playback=''):
    did = stable_id(key)
    positions = []
    for s,f,i in coordinates:
        positions.append(dict(position_id=stable_id(f'{key}/s{s}/f{f}'), interval=i,
                              note_name=spell(root,i), shape='star' if i=='R' else 'dot',
                              color='#EF4444' if i=='R' else None, string=s, fret=f,
                              custom_label=None, note=None))
    sequence = []
    ordered = sorted(positions, key=lambda p:(TUNING[6-p['string']]+p['fret'], -p['string']))
    if playback == 'chord':
        sequence = [dict(position_ids=[p['position_id'] for p in ordered],value=dict(num=1,den=1),strum='none')]
    elif playback == 'run':
        sequence = [dict(position_ids=[p['position_id']],value=dict(num=1,den=4),strum='none') for p in ordered]
    return dict(key=key, diagram_id=did, tier=tier, family=family, formula=formula,
                names=dict(en=en,pt_BR=pt), root_note=root, mode=mode,
                label_display='interval', color='#3B82F6', positions=positions,
                regions=[], sequence=sequence, tempo_bpm=60 if sequence else None,
                time_signature=dict(beats=4,beat_value=4),
                instruments=['guitar', 'electric-guitar'])


def voicings(root, intervals, strings, drop=()):
    """Realize closed chords and exact octave-drop transformations on fixed strings."""
    offsets = [semitones(i) for i in intervals]
    for inversion in range(len(offsets)):
        order = list(range(inversion,len(offsets))) + list(range(inversion))
        pitches = [offsets[i] + (12 if i<inversion else 0) for i in order]
        for from_top in drop: pitches[-from_top] -= 12
        voices = sorted(zip(pitches,order))
        for base in range(24,85):
            if base % 12 != pitch_class(root): continue
            out = [(s,base+v-TUNING[6-s],intervals[i]) for s,(v,i) in zip(strings,voices)]
            frets = [f for _,f,_ in out]
            if all(0<=f<=MAX_FRET for f in frets) and max(frets)-min(frets)<=5:
                yield inversion, base, out


def generate():
    result = []
    for root in ROOTS:
        rp = root_pt(root)
        for family, intervals, en, pt in [('chromatic',CHROMATIC,'Chromatic map','Mapa cromático'),('root',['R'],'Root map','Mapa de fundamentais')]:
            result.append(entry(f'{family}/{root}',root,intervals,cells(root,intervals),f'{root} {en} — Frets 0–12',f'{pt} de {rp} — Casas 0–12',family,'A'))
        for fk,(en,pt,formula,tier,mode) in FORMULAS.items():
            ints = formula.split()
            family = 'arpeggio' if fk in TRIADS+SEVENTHS or fk.startswith(('maj','dom','min')) and fk[-1].isdigit() else 'scale'
            result.append(entry(f'{family}/{fk}/{root}/frets-0-12',root,ints,cells(root,ints),f'{root} {en} — Frets 0–12',f'{pt} de {rp} — Casas 0–12',family,tier,fk,mode))
        # Pentatonic box coordinates in A minor / C major, transposed by complete octaves.
        for fk, reference in [('minor-pentatonic','A'),('major-pentatonic','C')]:
            en,pt,formula,tier,mode = FORMULAS[fk]; ints=formula.split()
            for box, template in enumerate(PENT_BOXES,1):
                for shift in range(-MAX_FRET,MAX_FRET+1):
                    if shift%12 != (pitch_class(root)-pitch_class(reference))%12: continue
                    coords=[(6-index,f+shift) for index,pair in enumerate(template) for f in pair]
                    if min(f for _,f in coords)<0 or max(f for _,f in coords)>MAX_FRET: continue
                    lo=min(f for _,f in coords)
                    result.append(entry(f'pentatonic-box/{fk}/{root}/{box}/{lo}',root,ints,tagged(root,ints,coords),f'{root} {en} — Box {box}, fret {lo}',f'{pt} de {rp} — Desenho {box}, casa {lo}','pentatonic-box','A',fk,mode,'run'))
        for fk in ['major','harmonic-minor','melodic-minor']:
            en,pt,formula,tier,mode=FORMULAS[fk]; ints=formula.split()
            pcmap={(pitch_class(root)+semitones(i))%12:i for i in ints}
            for degree, interval in enumerate(ints,1):
                for start in range(0,MAX_FRET+1):
                    first=TUNING[0]+start
                    if first%12 != (pitch_class(root)+semitones(interval))%12: continue
                    coords=[]; previous=first-1
                    for string in range(6,0,-1):
                        found=[]
                        for fret in range(MAX_FRET+1):
                            pitch=TUNING[6-string]+fret
                            if pitch>previous and pitch%12 in pcmap:
                                found.append((string,fret,pcmap[pitch%12]))
                                if len(found)==3: break
                        if len(found)!=3: break
                        coords.extend(found); previous=TUNING[6-string]+found[-1][1]
                    if len(coords)!=18: continue
                    result.append(entry(f'3nps/{fk}/{root}/{degree}/{start}',root,ints,coords,f'{root} {en} — 3NPS {degree}, fret {start}',f'{pt} de {rp} — 3 notas por corda, padrão {degree}, casa {start}','3nps','B',fk,mode,'run'))
        for shape,(reference,template) in CAGED.items():
            for shift in range(MAX_FRET+1):
                if shift%12 != (pitch_class(root)-pitch_class(reference))%12: continue
                coords=[(s,f+shift) for s,f in template]
                if max(f for _,f in coords)>MAX_FRET: continue
                ints=FORMULAS['major-triad'][2].split()
                result.append(entry(f'caged/{root}/{shape}/{shift}',root,ints,tagged(root,ints,coords),f'{root} major — CAGED {shape}, shift {shift}',f'Acorde maior de {rp} — CAGED {shape}, deslocamento {shift}','caged','A','major-triad','major','chord'))
                lo=min(f for _,f in coords); hi=max(f for _,f in coords)
                for fk in ['major','natural-minor','major-triad','minor-triad']:
                    en,pt,formula,tier,mode=FORMULAS[fk]; ints=formula.split()
                    result.append(entry(f'caged-window/{fk}/{root}/{shape}/{shift}',root,ints,cells(root,ints,lo,hi),f'{root} {en} — CAGED {shape} window, frets {lo}–{hi}',f'{pt} de {rp} — Região CAGED {shape}, casas {lo}–{hi}','caged-window','A',fk,mode))
        for system,ints in [('quartal-3','R 4 b7'),('quartal-4','R 4 b7 b3'),('quintal-3','R 5 9'),('quintal-4','R 5 9 13')]:
            degrees=ints.split()
            result.append(entry(f'structure/{system}/{root}',root,degrees,cells(root,degrees),f'{root} {system} — Interval map',f'Estrutura de {"quartas" if system.startswith("quartal") else "quintas"} de {rp} — {len(degrees)} notas, mapa de intervalos','interval-structure','C',system))
        for context in ['maj7','dom7','min7']:
            for fk in ['major-pentatonic','minor-pentatonic']:
                for offset in range(12):
                    pentroot=ROOTS[(pitch_class(root)+offset)%12]
                    pentints=FORMULAS[fk][2].split()
                    # Context-relative labels; names explicitly retain the pentatonic tonic.
                    offsets={(pitch_class(pentroot)+semitones(i)-pitch_class(root))%12 for i in pentints}
                    ints=[CHROMATIC[i] for i in sorted(offsets)]
                    result.append(entry(f'substitution/{context}/{root}/{fk}/{pentroot}',root,ints,cells(root,ints),f'{pentroot} {FORMULAS[fk][0]} over {root} {context} — Tension map',f'{FORMULAS[fk][1]} de {root_pt(pentroot)} sobre {rp} {context} — Mapa de tensões','substitution','C',fk))
        major=FORMULAS['major'][2].split()
        triads=[{i%7,(i+2)%7,(i+4)%7} for i in range(7)]
        for a,b in itertools.combinations(range(7),2):
            if triads[a]&triads[b]: continue
            ints=[major[i] for i in sorted(triads[a]|triads[b])]
            result.append(entry(f'triad-pair/{root}/{a+1}-{b+1}',root,ints,cells(root,ints),f'{root} major — Triad pair {a+1}/{b+1}, hexatonic map',f'Escala maior de {rp} — Par de tríades {a+1}/{b+1}, mapa hexatônico','triad-pair','C'))
        for degrees in [('R','2','3','5'),('R','3','4','5')]:
            key='-'.join(degrees)
            result.append(entry(f'digital/{root}/{key}',root,list(degrees),cells(root,list(degrees)),f'{root} major — Digital pattern {key}, note map',f'Escala maior de {rp} — Padrão {key}, mapa de notas','digital-pattern','C'))
    result.sort(key=lambda e:e['key'])
    validate(result)
    return result


def validate(entries):
    seen=set()
    for e in entries:
        if e['diagram_id'] in seen: raise ValueError('Duplicate diagram ID')
        seen.add(e['diagram_id'])
        if e['instruments'] != ['guitar', 'electric-guitar']: raise ValueError('Invalid compatible instruments')
        if set(e['names'])!={'en','pt_BR'} or any(not s or len(s)>200 for s in e['names'].values()): raise ValueError('Incomplete localized names')
        coords=set(); ids=set()
        if not e['positions']: raise ValueError('Empty diagram')
        for p in e['positions']:
            cell=(p['string'],p['fret'])
            if cell in coords or not 1<=cell[0]<=6 or not 0<=cell[1]<=MAX_FRET: raise ValueError(f'Invalid cell {e["key"]}: {cell}')
            coords.add(cell); ids.add(p['position_id'])
            if p['interval'] not in INTERVALS: raise ValueError('Unsupported interval')
            expected=(TUNING[6-p['string']]+p['fret'])%12
            if expected!=pitch_class(p['note_name']) or expected!=(pitch_class(e['root_note'])+semitones(p['interval']))%12: raise ValueError('Pitch mismatch')
            for field in ['custom_label','note']:
                if p[field] is not None and set(p[field])!={'en','pt_BR'}: raise ValueError('Incomplete annotation')
        if bool(e['sequence']) != (e['tempo_bpm'] is not None): raise ValueError('Tempo/sequence mismatch')
        for step in e['sequence']:
            if not set(step['position_ids'])<=ids: raise ValueError('Broken playback reference')


def sql_text(value):
    return "'"+value.replace("'","''")+"'"


def compact(value):
    return json.dumps(value,ensure_ascii=False,sort_keys=True,separators=(',',':'))


def batches(values, size):
    for start in range(0, len(values), size):
        yield values[start:start + size]


def render_sql(entries):
    sql=["-- Frozen bilingual guitar catalog. The fixed system profile owns every basic diagram.",
         "-- Each assertion deliberately divides by zero when its precondition is false.",
         "SELECT 1 / (SELECT CASE WHEN EXISTS (SELECT 1 FROM languages WHERE code='en') AND EXISTS (SELECT 1 FROM languages WHERE code='pt_BR') AND NOT EXISTS (SELECT 1 FROM languages WHERE code NOT IN ('en','pt_BR','any')) THEN 1 ELSE 0 END) AS catalog_translations_required;",
         "SELECT 1 / (SELECT CASE WHEN (NOT EXISTS (SELECT 1 FROM users WHERE id="+sql_text(SYSTEM_CATALOG_USER_ID)+" OR clerk_user_id="+sql_text(SYSTEM_CATALOG_CLERK_USER_ID)+")) OR EXISTS (SELECT 1 FROM users u JOIN languages l ON l.id=u.locale_id WHERE u.id="+sql_text(SYSTEM_CATALOG_USER_ID)+" AND u.clerk_user_id="+sql_text(SYSTEM_CATALOG_CLERK_USER_ID)+" AND u.role='admin' AND u.display_name="+sql_text(SYSTEM_CATALOG_DISPLAY_NAME)+" AND l.code='en') THEN 1 ELSE 0 END) AS system_catalog_profile_compatible;",
         "INSERT INTO users (id,clerk_user_id,role,display_name,locale_id,registered_at) SELECT "+sql_text(SYSTEM_CATALOG_USER_ID)+","+sql_text(SYSTEM_CATALOG_CLERK_USER_ID)+",'admin',"+sql_text(SYSTEM_CATALOG_DISPLAY_NAME)+",id,'2026-10-01T00:00:00Z' FROM languages WHERE code='en' AND NOT EXISTS (SELECT 1 FROM users WHERE id="+sql_text(SYSTEM_CATALOG_USER_ID)+");",
         "SELECT 1 / (SELECT CASE WHEN (SELECT count(*) FROM instruments WHERE names IN ('{\"en\":\"Guitar\",\"pt_BR\":\"Violão\"}'::jsonb,'{\"en\":\"Electric guitar\",\"pt_BR\":\"Guitarra elétrica\"}'::jsonb)) <= 2 THEN 1 ELSE 0 END) AS unambiguous_catalog_instruments_required;",
         "SELECT 1 / (SELECT CASE WHEN (SELECT count(*) FROM skills WHERE name='Scales' AND parent_id IS NULL) <= 1 AND (SELECT count(*) FROM concepts WHERE name='Fretboard patterns' AND parent_id IS NULL) <= 1 THEN 1 ELSE 0 END) AS unambiguous_catalog_classification_required;",
         "INSERT INTO skills (id,name) SELECT '"+stable_id('skill/scales')+"','Scales' WHERE NOT EXISTS (SELECT 1 FROM skills WHERE name='Scales' AND parent_id IS NULL);",
         "INSERT INTO concepts (id,name) SELECT '"+stable_id('concept/fretboard-patterns')+"','Fretboard patterns' WHERE NOT EXISTS (SELECT 1 FROM concepts WHERE name='Fretboard patterns' AND parent_id IS NULL);"]
    for key, names in INSTRUMENTS.items():
        names_json = sql_text(compact(names)) + '::jsonb'
        sql.append("INSERT INTO instruments (id,names,family,string_count,tuning,default_voice_id) SELECT "+sql_text(stable_id('instrument/'+key))+","+names_json+",'fretted',6,'[\"E2\",\"A2\",\"D3\",\"G3\",\"B3\",\"E4\"]','acoustic-guitar' WHERE NOT EXISTS (SELECT 1 FROM instruments WHERE names="+names_json+");")
    for skill in ['Chords', 'Arpeggios', 'Improvisation', 'Fretboard navigation']:
        sql.append("SELECT 1 / (SELECT CASE WHEN (SELECT count(*) FROM skills WHERE name="+sql_text(skill)+" AND parent_id IS NULL) <= 1 THEN 1 ELSE 0 END) AS unambiguous_skill_required;")
        sql.append("INSERT INTO skills (id,name) SELECT "+sql_text(stable_id('skill/'+skill))+","+sql_text(skill)+" WHERE NOT EXISTS (SELECT 1 FROM skills WHERE name="+sql_text(skill)+" AND parent_id IS NULL);")

    diagrams = []
    diagram_instruments = []
    positions = []
    classifications = []
    for e in entries:
        instrument_names = sql_text(compact(INSTRUMENTS['guitar'])) + '::jsonb'
        instrument_id = "(SELECT id FROM instruments WHERE names="+instrument_names+" AND family='fretted' AND string_count=6 AND tuning='[\"E2\",\"A2\",\"D3\",\"G3\",\"B3\",\"E4\"]'::jsonb)"
        values=[sql_text(e['diagram_id']), instrument_id,sql_text(compact(e['names'])),"'basic'",sql_text(SYSTEM_CATALOG_USER_ID),sql_text(e['root_note']),"'interval'","'#3B82F6'",sql_text(e['mode']) if e['mode'] else 'NULL',str(e['tempo_bpm']) if e['tempo_bpm'] else 'NULL','4','4',sql_text(compact(e['sequence'])),"'2026-10-01T00:00:00Z'"]
        diagrams.append('('+','.join(values)+')')
        for key in e['instruments']:
            compatible_names = sql_text(compact(INSTRUMENTS[key])) + '::jsonb'
            compatible_id = "(SELECT id FROM instruments WHERE names="+compatible_names+" AND family='fretted' AND string_count=6 AND tuning='[\"E2\",\"A2\",\"D3\",\"G3\",\"B3\",\"E4\"]'::jsonb)"
            diagram_instruments.append('('+sql_text(e['diagram_id'])+'::uuid,'+compatible_id+')')
        for ordinal,p in enumerate(e['positions']):
            positions.append('('+','.join([sql_text(p['position_id']),sql_text(e['diagram_id']),str(ordinal),sql_text(p['interval']),sql_text(p['note_name']),sql_text(p['shape']),sql_text(p['color']) if p['color'] else 'NULL',str(p['string']),str(p['fret'])])+')')
        skill = 'Chords' if e['family'] in ['caged','triad-inversion','seventh-inversion','drop-2','drop-3','drop-2-4','shell'] else 'Arpeggios' if e['family']=='arpeggio' else 'Improvisation' if e['tier']=='C' else 'Fretboard navigation' if e['family'] in ['root','chromatic'] else 'Scales'
        classifications.append((e['diagram_id'], skill))
    for group in batches(diagrams, 250):
        sql.append('INSERT INTO diagrams (id,instrument_id,names,kind,created_by,root_note,label_display,color,mode,tempo_bpm,time_signature_beats,time_signature_beat_value,sequence,created_at) VALUES '+','.join(group)+';')
    for group in batches(diagram_instruments, 1000):
        sql.append('INSERT INTO diagram_instruments (diagram_id,instrument_id) VALUES '+','.join(group)+';')
    for group in batches(positions, 1000):
        sql.append('INSERT INTO positions (id,diagram_id,ordinal,interval,note_name,shape,color,string_number,fret) VALUES '+','.join(group)+';')
    for group in batches(classifications, 1000):
        values=','.join('('+sql_text(diagram_id)+'::uuid,'+sql_text(skill)+')' for diagram_id,skill in group)
        sql.append("INSERT INTO diagram_skills (diagram_id,skill_id,linked_at) SELECT v.diagram_id,s.id,'2026-10-01T00:00:00Z' FROM (VALUES "+values+") AS v(diagram_id,skill_name) JOIN skills s ON s.name=v.skill_name AND s.parent_id IS NULL;")
    for group in batches(classifications, 1000):
        values=','.join('('+sql_text(diagram_id)+'::uuid)' for diagram_id,_ in group)
        sql.append("INSERT INTO diagram_concepts (diagram_id,concept_id,linked_at) SELECT v.diagram_id,c.id,'2026-10-01T00:00:00Z' FROM (VALUES "+values+") AS v(diagram_id) JOIN concepts c ON c.name='Fretboard patterns' AND c.parent_id IS NULL;")
    return '\n'.join(sql)+'\n'


def main():
    parser=argparse.ArgumentParser(); parser.add_argument('--output',type=Path,required=True)
    args=parser.parse_args(); entries=generate(); args.output.mkdir(parents=True,exist_ok=True)
    payload=compact(entries)+'\n'; sql=render_sql(entries)
    (args.output/'catalog.json').write_text(payload)
    (args.output/'20261001000000_basic_guitar_catalog.up.sql').write_text(sql)
    report=dict(diagrams=len(entries),positions=sum(len(e['positions']) for e in entries),tiers=dict(sorted(Counter(e['tier'] for e in entries).items())),families=dict(sorted(Counter(e['family'] for e in entries).items())),sha256=hashlib.sha256(payload.encode()).hexdigest())
    (args.output/'coverage.json').write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n')
    print(json.dumps(report,ensure_ascii=False,indent=2))


if __name__=='__main__': main()
