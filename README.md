# UndeSchimb

UndeSchimb compară cursuri valutare din România și raportează rezultatul la reperul BNR. Interfața oferă atât un clasament general, cât și file separate pentru bănci, brokeri/fintech și case de schimb, iar avantajele condiționate sunt explicate fără a fi confundate cu ofertele standard.

## Pornire locală

```bash
docker compose up --build
```

Aplicația este disponibilă la `http://localhost:5173`, iar API-ul la `http://localhost:8080`.

La pornire, serviciul colectează cursurile BNR și cele publicate de furnizori, apoi reîncearcă la fiecare 15 minute. O sursă care nu poate fi interpretată nu înlocuiește ultima valoare validă; interfața o va afișa ca expirată.

Poți copia `.env.example` în `.env` pentru a declara zile suplimentare fără tranzacționare XTB. Pentru a transmite aceste variabile în Docker Compose, adaugă-le la serviciul `api` sau folosește un fișier Compose override.

## API

- `GET /api/v1/comparison?from=RON&to=EUR&amount=1000`
- `GET /api/v1/history?provider=bcr&currency=EUR&side=sell&period=30`
- `GET /healthz`

Ratele BNR sunt un reper informativ, nu o ofertă executabilă. XTB și Revolut sunt marcate ca estimative: aplicația colectează cotațiile publice Standard și explică separat taxele și limitele care pot modifica rezultatul executabil. TradeVille este prezentat informativ deoarece cotația exactă trebuie confirmată în platformă; nu este fabricată o valoare pentru clasament.

Pentru numerar sunt colectate Tavex și Luxor București. Interfața arată explicit politica locațiilor: Tavex publică același curs pentru toate birourile, iar Luxor publică valori diferite pentru București și Arad. Cursul Luxor intră în clasament numai când suma depășește pragul oficial de 500 EUR sau echivalent.

Cursul avantajos ING și Raiffeisen Smart Hour apar în carduri separate, împreună cu condițiile de eligibilitate. Smart Hour intră într-un calcul numeric numai pentru EUR, în intervalul activ și în limita zilnică publicată; limita lunară trebuie verificată de utilizator.
