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

Istoricul acceptă perioade de 7, 30 sau 90 de zile și returnează ultima cotație colectată în fiecare zi calendaristică din București. Dacă lipsește colectarea BNR pentru o zi, observația furnizorului rămâne disponibilă, iar `bnr_rate` este `null`. Graficul nu înlocuiește aceste goluri cu zero sau cu un curs BNR mai vechi.

Ratele BNR sunt un reper informativ, nu o ofertă executabilă. XTB și Revolut sunt marcate ca estimative: aplicația colectează cotațiile publice Standard și explică separat taxele și limitele care pot modifica rezultatul executabil. Pentru EUR/RON, TradeVille este estimat transparent în jurul BNR cu jumătate din spread-ul maxim publicat de 50 pips pe fiecare sens; cotația executabilă trebuie confirmată în platformă.

Pentru numerar sunt colectate Tavex și Luxor București. Interfața arată explicit politica locațiilor: Tavex publică același curs pentru toate birourile, iar Luxor publică valori diferite pentru București și Arad. Cursul Luxor intră în clasament numai când suma depășește pragul oficial de 500 EUR sau echivalent.

Cursurile ING avantajos, Raiffeisen Smart Hour și BRD YOU au comutatoare explicite. Ofertele inactive pot fi previzualizate, dar sunt marcate ca indisponibile și nu primesc eticheta de cea mai bună ofertă disponibilă. Beneficiile fără cotație publică — precum cursul preferențial BCR în George și cursul negociat BT — sunt explicate fără a inventa o valoare. Cursul digital CEC este deja inclus prin tabela sa publică de online banking.
