#!/usr/bin/env python3
"""
Generate sample CSV datasets for sql.garden.
Run from the sql.garden project root:  python3 scripts/gen_sample_data.py
"""
import csv, math, os, random
from datetime import date, timedelta

random.seed(42)

BASE = os.path.join(os.path.dirname(os.path.abspath(__file__)), '..', 'sampledata')

def write(rel, rows, cols=None):
    path = os.path.join(BASE, rel)
    os.makedirs(os.path.dirname(path), exist_ok=True)
    if not rows: return
    cols = cols or list(rows[0].keys())
    with open(path, 'w', newline='', encoding='utf-8') as f:
        w = csv.DictWriter(f, fieldnames=cols)
        w.writeheader()
        w.writerows(rows)
    print(f'  {len(rows):6,d} rows  →  {rel}')

# ── F1 2023 ───────────────────────────────────────────────────────────────────

DRIVERS = [
    (1,  'VER', 'Max Verstappen',    'Dutch',       'Red Bull Racing',  1),
    (2,  'PER', 'Sergio Perez',      'Mexican',     'Red Bull Racing', 11),
    (3,  'ALO', 'Fernando Alonso',   'Spanish',     'Aston Martin',    14),
    (4,  'HAM', 'Lewis Hamilton',    'British',     'Mercedes',        44),
    (5,  'LEC', 'Charles Leclerc',   'Monégasque',  'Ferrari',         16),
    (6,  'SAI', 'Carlos Sainz',      'Spanish',     'Ferrari',         55),
    (7,  'NOR', 'Lando Norris',      'British',     'McLaren',          4),
    (8,  'RUS', 'George Russell',    'British',     'Mercedes',        63),
    (9,  'PIA', 'Oscar Piastri',     'Australian',  'McLaren',         81),
    (10, 'STR', 'Lance Stroll',      'Canadian',    'Aston Martin',    18),
    (11, 'GAS', 'Pierre Gasly',      'French',      'Alpine',          10),
    (12, 'OCO', 'Esteban Ocon',      'French',      'Alpine',          31),
    (13, 'ALB', 'Alexander Albon',   'Thai',        'Williams',        23),
    (14, 'TSU', 'Yuki Tsunoda',      'Japanese',    'AlphaTauri',      22),
    (15, 'BOT', 'Valtteri Bottas',   'Finnish',     'Alfa Romeo',      77),
    (16, 'ZHO', 'Guanyu Zhou',       'Chinese',     'Alfa Romeo',      24),
    (17, 'HUL', 'Nico Hülkenberg',  'German',      'Haas',            27),
    (18, 'MAG', 'Kevin Magnussen',   'Danish',      'Haas',            20),
    (19, 'SAR', 'Logan Sargeant',    'American',    'Williams',         2),
    (20, 'RIC', 'Daniel Ricciardo',  'Australian',  'AlphaTauri',       3),
]

TEAMS = [
    (1,  'Red Bull Racing', 'Oracle Red Bull Racing',         'Austrian',  'Honda RBPT', 6),
    (2,  'Mercedes',        'Mercedes-AMG Petronas F1 Team',  'German',    'Mercedes',   8),
    (3,  'Ferrari',         'Scuderia Ferrari',               'Italian',   'Ferrari',   16),
    (4,  'McLaren',         'McLaren F1 Team',                'British',   'Mercedes',   8),
    (5,  'Aston Martin',    'Aston Martin Aramco F1 Team',    'British',   'Mercedes',   0),
    (6,  'Alpine',          'BWT Alpine F1 Team',             'French',    'Renault',    2),
    (7,  'Williams',        'Williams Racing',                'British',   'Mercedes',   9),
    (8,  'AlphaTauri',      'Scuderia AlphaTauri',            'Italian',   'Honda RBPT', 1),
    (9,  'Alfa Romeo',      'Stake F1 Team Kick Sauber',      'Swiss',     'Ferrari',    1),
    (10, 'Haas',            'MoneyGram Haas F1 Team',         'American',  'Ferrari',    0),
]

RACES = [
    (1,  'Bahrain Grand Prix',       'Bahrain International Circuit',   'Bahrain',      '2023-03-05'),
    (2,  'Saudi Arabian Grand Prix', 'Jeddah Corniche Circuit',         'Saudi Arabia', '2023-03-19'),
    (3,  'Australian Grand Prix',    'Albert Park Circuit',             'Australia',    '2023-03-30'),
    (4,  'Azerbaijan Grand Prix',    'Baku City Circuit',               'Azerbaijan',   '2023-04-30'),
    (5,  'Miami Grand Prix',         'Miami International Autodrome',   'USA',          '2023-05-07'),
    (6,  'Monaco Grand Prix',        'Circuit de Monaco',               'Monaco',       '2023-05-28'),
    (7,  'Spanish Grand Prix',       'Circuit de Barcelona-Catalunya',  'Spain',        '2023-06-04'),
    (8,  'Canadian Grand Prix',      'Circuit Gilles Villeneuve',       'Canada',       '2023-06-18'),
    (9,  'Austrian Grand Prix',      'Red Bull Ring',                   'Austria',      '2023-07-02'),
    (10, 'British Grand Prix',       'Silverstone Circuit',             'UK',           '2023-07-09'),
    (11, 'Hungarian Grand Prix',     'Hungaroring',                     'Hungary',      '2023-07-23'),
    (12, 'Belgian Grand Prix',       'Circuit de Spa-Francorchamps',    'Belgium',      '2023-07-30'),
    (13, 'Dutch Grand Prix',         'Circuit Zandvoort',               'Netherlands',  '2023-08-27'),
    (14, 'Italian Grand Prix',       'Autodromo Nazionale Monza',       'Italy',        '2023-09-03'),
    (15, 'Singapore Grand Prix',     'Marina Bay Street Circuit',       'Singapore',    '2023-09-17'),
    (16, 'Japanese Grand Prix',      'Suzuka International Circuit',    'Japan',        '2023-09-24'),
    (17, 'Qatar Grand Prix',         'Lusail International Circuit',    'Qatar',        '2023-10-08'),
    (18, 'United States Grand Prix', 'Circuit of the Americas',         'USA',          '2023-10-22'),
    (19, 'Mexico City Grand Prix',   'Autodromo Hermanos Rodriguez',    'Mexico',       '2023-10-29'),
    (20, 'São Paulo Grand Prix',     'Autodromo José Carlos Pace',      'Brazil',       '2023-11-05'),
    (21, 'Las Vegas Grand Prix',     'Las Vegas Strip Circuit',         'USA',          '2023-11-18'),
    (22, 'Abu Dhabi Grand Prix',     'Yas Marina Circuit',              'UAE',          '2023-11-26'),
]

POINTS_MAP = {1:25,2:18,3:15,4:12,5:10,6:8,7:6,8:4,9:2,10:1}

# Relative driver pace (higher = faster). Verstappen much stronger than field.
PACE = {
    1:0.970, 2:0.925, 3:0.900, 4:0.910, 5:0.905, 6:0.900,
    7:0.895, 8:0.905, 9:0.870, 10:0.855, 11:0.840, 12:0.838,
    13:0.825, 14:0.820, 15:0.815, 16:0.812, 17:0.818, 18:0.815,
    19:0.800, 20:0.830,
}

def gen_f1():
    print('\nF1 2023')

    write('f1/drivers.csv', [
        {'driver_id':d[0],'code':d[1],'name':d[2],'nationality':d[3],'team':d[4],'car_number':d[5]}
        for d in DRIVERS
    ])
    write('f1/teams.csv', [
        {'team_id':t[0],'name':t[1],'full_name':t[2],'nationality':t[3],'engine':t[4],'championships':t[5]}
        for t in TEAMS
    ])
    write('f1/races.csv', [
        {'race_id':r[0],'season':2023,'round':r[0],'name':r[1],'circuit':r[2],'country':r[3],'date':r[4]}
        for r in RACES
    ])

    results = []
    rid = 1
    for race in RACES:
        race_id = race[0]
        base_laps = random.randint(52, 71)
        # Qualifying: sort by pace + noise → grid
        grid = sorted(range(1, 21), key=lambda d: -(PACE[d] + random.gauss(0, 0.018)))
        # DNFs: ~2–3 per race
        dnf_count = random.choices([0,1,2,3,4], weights=[5,25,40,20,10])[0]
        dnfs = set(random.sample(grid[5:], min(dnf_count, len(grid)-5)))  # avoid top 5 DNF too often

        finishers = [d for d in grid if d not in dnfs]
        # Race order: pace + noise
        finish_order = sorted(finishers, key=lambda d: -(PACE[d] + random.gauss(0, 0.015)))

        fl_driver = random.choice(finish_order[:min(10,len(finish_order))])
        fl_ms = random.randint(79_000, 97_000)

        pos = 1
        for d in finish_order:
            g = grid.index(d) + 1
            pts = float(POINTS_MAP.get(pos, 0))
            is_fl = (d == fl_driver) and pos <= 10
            if is_fl:
                pts += 1.0
            laps = base_laps if pos == 1 else base_laps - (1 if pos > 15 else 0)
            results.append({
                'result_id': rid, 'race_id': race_id, 'driver_id': d,
                'grid': g, 'position': pos, 'points': pts,
                'laps': laps, 'status': 'Finished',
                'fastest_lap': is_fl, 'fastest_lap_ms': fl_ms if is_fl else '',
            })
            pos += 1
            rid += 1

        for d in dnfs:
            g = grid.index(d) + 1
            results.append({
                'result_id': rid, 'race_id': race_id, 'driver_id': d,
                'grid': g, 'position': '', 'points': 0.0,
                'laps': random.randint(3, base_laps - 5),
                'status': random.choice(['DNF','DNF','DNF','Accident','Mechanical']),
                'fastest_lap': False, 'fastest_lap_ms': '',
            })
            rid += 1

    write('f1/results.csv', results)

# ── Markets ───────────────────────────────────────────────────────────────────

TICKERS = [
    ('AAPL', 'Apple Inc.',           'Technology',    'USA',  73.0,  0.25, 0.30),
    ('MSFT', 'Microsoft Corp.',      'Technology',    'USA', 158.0,  0.22, 0.28),
    ('GOOGL','Alphabet Inc.',        'Technology',    'USA',  67.0,  0.20, 0.30),
    ('AMZN', 'Amazon.com Inc.',      'Consumer',      'USA',  94.0,  0.28, 0.35),
    ('TSLA', 'Tesla Inc.',           'Automotive',    'USA',  28.0,  0.35, 0.65),
    ('NVDA', 'NVIDIA Corp.',         'Technology',    'USA',  60.0,  0.40, 0.55),
    ('JPM',  'JPMorgan Chase',       'Finance',       'USA', 102.0,  0.15, 0.25),
    ('V',    'Visa Inc.',            'Finance',       'USA', 190.0,  0.18, 0.22),
    ('JNJ',  'Johnson & Johnson',    'Healthcare',    'USA', 146.0,  0.08, 0.15),
    ('XOM',  'Exxon Mobil Corp.',    'Energy',        'USA',  45.0,  0.20, 0.30),
]

def gen_markets():
    print('\nMarkets')
    write('markets/companies.csv', [
        {'ticker':t[0],'name':t[1],'sector':t[2],'country':t[3]}
        for t in TICKERS
    ])

    start = date(2020, 1, 2)
    end   = date(2024, 12, 31)
    prices = []
    for ticker, _, _, _, s0, mu, sigma in TICKERS:
        price = s0
        dt = 1/252
        d = start
        while d <= end:
            if d.weekday() < 5:  # trading days only
                ret = random.gauss(mu * dt, sigma * math.sqrt(dt))
                price *= math.exp(ret)
                daily_vol = price * sigma * math.sqrt(dt)
                high  = price + abs(random.gauss(0, daily_vol))
                low   = price - abs(random.gauss(0, daily_vol))
                open_ = price + random.gauss(0, daily_vol * 0.4)
                open_ = max(low * 0.999, min(high * 1.001, open_))
                vol   = int(random.lognormvariate(math.log(5_000_000), 0.8))
                prices.append({
                    'date': d.isoformat(), 'ticker': ticker,
                    'open':  round(open_, 2), 'high': round(high, 2),
                    'low':   round(low,   2), 'close': round(price, 2),
                    'volume': vol,
                })
            d += timedelta(days=1)
    write('markets/prices.csv', prices)

# ── Economy ───────────────────────────────────────────────────────────────────

def gen_economy():
    print('\nEconomy')
    rows = []
    start = date(2014, 1, 1)
    sp500 = 1845.0
    ffr   = 0.07   # fed funds rate %
    unm   = 6.6    # unemployment %
    cpi   = 1.6    # CPI YoY %

    for m in range(132):   # 11 years × 12 months
        d = date(start.year + (start.month - 1 + m) // 12,
                 (start.month - 1 + m) % 12 + 1, 1)
        yr, mo = d.year, d.month

        # Fed funds rate trajectory
        if yr < 2015 or (yr == 2015 and mo < 12): ffr = max(0.07, ffr)
        elif yr == 2015 and mo == 12: ffr = 0.25
        elif yr in (2016,2017): ffr = min(1.50, ffr + random.gauss(0.06,0.03))
        elif yr == 2018: ffr = min(2.50, ffr + random.gauss(0.07,0.02))
        elif yr == 2019: ffr = max(1.50, ffr - random.gauss(0.05,0.02))
        elif yr == 2020 and mo <= 3: ffr = 0.09
        elif yr in (2020,2021): ffr = max(0.07, ffr + random.gauss(0,0.01))
        elif yr == 2022: ffr = min(4.50, ffr + random.gauss(0.5,0.1))
        elif yr == 2023: ffr = min(5.50, ffr + random.gauss(0.15,0.05))
        elif yr == 2024: ffr = max(5.25, ffr - random.gauss(0.05,0.02))
        ffr = round(max(0.0, ffr), 2)

        # Unemployment
        if yr == 2020 and mo == 4: unm = 14.7
        elif yr == 2020 and mo > 4: unm = max(6.7, unm - random.gauss(0.6,0.2))
        elif yr == 2021: unm = max(3.9, unm - random.gauss(0.25,0.15))
        elif yr in (2022,2023,2024): unm = max(3.4, unm + random.gauss(-0.02,0.08))
        else: unm = max(3.5, unm + random.gauss(-0.02,0.1))
        unm = round(max(0.0, unm), 1)

        # CPI
        if yr < 2021: cpi = max(0.1, cpi + random.gauss(0.0, 0.15))
        elif yr == 2021: cpi = min(7.5, cpi + random.gauss(0.55,0.15))
        elif yr == 2022 and mo <= 6: cpi = min(9.1, cpi + random.gauss(0.4,0.1))
        elif yr == 2022: cpi = max(6.5, cpi - random.gauss(0.35,0.1))
        elif yr == 2023: cpi = max(3.0, cpi - random.gauss(0.3,0.1))
        elif yr == 2024: cpi = max(2.4, cpi - random.gauss(0.1,0.05))
        cpi = round(max(0.0, cpi), 1)

        # GDP growth (annualized %)
        if yr == 2020 and mo in (4,5,6): gdp = round(random.gauss(-25, 3), 1)
        elif yr == 2020 and mo in (7,8,9): gdp = round(random.gauss(25, 4), 1)
        elif yr == 2020: gdp = round(random.gauss(-2, 1.5), 1)
        elif yr == 2021: gdp = round(random.gauss(5.5, 1.2), 1)
        elif yr == 2022: gdp = round(random.gauss(2.1, 0.9), 1)
        elif yr == 2023: gdp = round(random.gauss(2.5, 0.7), 1)
        elif yr == 2024: gdp = round(random.gauss(2.8, 0.5), 1)
        else: gdp = round(random.gauss(2.3, 0.8), 1)

        # S&P 500 random walk with regime
        sp_mu = 0.12/12
        if yr == 2020 and mo in (2,3): sp_mu = -0.18
        elif yr == 2020 and mo > 3: sp_mu = 0.06
        sp500 *= math.exp(random.gauss(sp_mu, 0.035))
        sp500 = round(max(2000, sp500), 2)

        rows.append({
            'date': d.isoformat(),
            'gdp_growth_pct': gdp,
            'unemployment_pct': unm,
            'cpi_yoy_pct': cpi,
            'fed_funds_rate': ffr,
            'sp500_close': sp500,
        })
    write('economy/indicators.csv', rows)

# ── E-commerce ────────────────────────────────────────────────────────────────

def gen_ecommerce():
    print('\nE-commerce')
    users = [
        {'user_id':1,'name':'Alice Chen',   'email':'alice@example.com',  'city':'San Francisco','country':'USA',       'plan':'pro',     'joined':'2023-01-15'},
        {'user_id':2,'name':'Bob Smith',    'email':'bob@example.com',    'city':'New York',     'country':'USA',       'plan':'starter', 'joined':'2023-02-20'},
        {'user_id':3,'name':'Carol Davis',  'email':'carol@example.com',  'city':'London',       'country':'UK',        'plan':'pro',     'joined':'2023-03-05'},
        {'user_id':4,'name':'Dave Wilson',  'email':'dave@example.com',   'city':'Toronto',      'country':'Canada',    'plan':'starter', 'joined':'2023-04-10'},
        {'user_id':5,'name':'Eve Martinez', 'email':'eve@example.com',    'city':'Madrid',       'country':'Spain',     'plan':'enterprise','joined':'2023-05-22'},
        {'user_id':6,'name':'Frank Lee',    'email':'frank@example.com',  'city':'Tokyo',        'country':'Japan',     'plan':'pro',     'joined':'2023-06-01'},
        {'user_id':7,'name':'Grace Kim',    'email':'grace@example.com',  'city':'Seoul',        'country':'S. Korea',  'plan':'starter', 'joined':'2023-07-14'},
        {'user_id':8,'name':'Hiro Tanaka',  'email':'hiro@example.com',   'city':'Osaka',        'country':'Japan',     'plan':'pro',     'joined':'2023-08-03'},
        {'user_id':9,'name':'Isla Brown',   'email':'isla@example.com',   'city':'Sydney',       'country':'Australia', 'plan':'enterprise','joined':'2023-09-19'},
        {'user_id':10,'name':'Jack Taylor', 'email':'jack@example.com',   'city':'Chicago',      'country':'USA',       'plan':'starter', 'joined':'2023-10-07'},
    ]
    products = [
        {'product_id':1, 'name':'Ergonomic Chair',     'category':'Furniture', 'price':349.99,'stock':42},
        {'product_id':2, 'name':'Standing Desk',       'category':'Furniture', 'price':599.00,'stock':18},
        {'product_id':3, 'name':'Monitor 27"',         'category':'Displays',  'price':449.95,'stock':31},
        {'product_id':4, 'name':'Mechanical Keyboard', 'category':'Peripherals','price':129.99,'stock':85},
        {'product_id':5, 'name':'USB-C Hub',           'category':'Peripherals','price': 49.99,'stock':120},
        {'product_id':6, 'name':'Webcam 4K',           'category':'Peripherals','price': 89.99,'stock':55},
        {'product_id':7, 'name':'Noise-Cancel Headphones','category':'Audio',  'price':299.99,'stock':38},
        {'product_id':8, 'name':'Laptop Stand',        'category':'Furniture', 'price': 79.99,'stock':92},
        {'product_id':9, 'name':'LED Desk Lamp',       'category':'Lighting',  'price': 54.99,'stock':74},
        {'product_id':10,'name':'Cable Management Kit','category':'Accessories','price': 19.99,'stock':200},
    ]
    orders = []
    items = []
    oid = 1
    iid = 1
    for month in range(1, 13):
        n = random.randint(3, 7)
        for _ in range(n):
            uid = random.randint(1, 10)
            day = random.randint(1, 28)
            created = f'2024-{month:02d}-{day:02d}'
            status = random.choices(['completed','shipped','pending','cancelled'],[60,20,15,5])[0]
            n_items = random.randint(1, 4)
            chosen = random.sample(products, n_items)
            total = sum(p['price'] * random.randint(1,3) for p in chosen)
            orders.append({'order_id':oid,'user_id':uid,'status':status,'total':round(total,2),'created_at':created})
            for p in chosen:
                qty = random.randint(1, 3)
                items.append({'item_id':iid,'order_id':oid,'product_id':p['product_id'],'quantity':qty,'unit_price':p['price']})
                iid += 1
            oid += 1
    write('ecommerce/users.csv',    users)
    write('ecommerce/products.csv', products)
    write('ecommerce/orders.csv',   orders)
    write('ecommerce/items.csv',    items)

# ── Weather ───────────────────────────────────────────────────────────────────

CITIES = [
    # city, country, lat_sign, base_winter_c, amplitude_c, precip_scale
    ('New York',  'USA',       1,  0.0, 14.0, 1.0),
    ('London',    'UK',        1,  5.5,  8.5, 1.3),
    ('Tokyo',     'Japan',     1,  4.0, 14.5, 1.1),
    ('Sydney',    'Australia',-1, 19.0, 10.0, 0.8),
    ('Cairo',     'Egypt',     1, 14.0, 13.0, 0.15),
]

COND_HOT  = ['Sunny','Sunny','Sunny','Partly Cloudy','Hazy']
COND_COLD = ['Cloudy','Overcast','Drizzle','Snow','Partly Cloudy','Sunny']
COND_MID  = ['Partly Cloudy','Sunny','Cloudy','Light Rain','Clear']

def gen_weather():
    print('\nWeather')
    rows = []
    start = date(2021, 1, 1)
    end   = date(2023, 12, 31)
    d = start
    while d <= end:
        for city, country, sign, base, amp, ps in CITIES:
            # day-of-year → seasonal avg temp
            doy = d.timetuple().tm_yday
            seasonal = base + sign * amp * math.sin((doy - 80) * 2 * math.pi / 365)
            high = round(seasonal + random.gauss(3, 2.5), 1)
            low  = round(seasonal - random.gauss(5, 2.0), 1)
            low  = min(low, high - 2)
            precip = round(max(0, random.lognormvariate(-1.5, 1.8) * ps), 1)
            hum   = round(min(99, max(20, 60 + random.gauss(0, 20) - (high - 15) * 0.5)), 1)
            if high > 28: cond = random.choice(COND_HOT)
            elif high < 8: cond = random.choice(COND_COLD)
            else:          cond = random.choice(COND_MID)
            if precip > 5: cond = 'Heavy Rain' if high > 5 else 'Snowfall'
            rows.append({
                'date': d.isoformat(), 'city': city, 'country': country,
                'temp_high_c': high, 'temp_low_c': low,
                'precipitation_mm': precip, 'humidity_pct': hum, 'condition': cond,
            })
        d += timedelta(days=1)
    write('weather/daily.csv', rows)

# ── Run ───────────────────────────────────────────────────────────────────────

gen_f1()
gen_markets()
gen_economy()
gen_ecommerce()
gen_weather()
print('\nDone.')
