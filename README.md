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

## Pagini de comparație și HTML cu date

Pagina principală și opt pagini de comparație sunt randate de serverul frontend Node, folosind același API ca aplicația. Cursurile, sumele primite, condițiile, sursele și orele colectării se găsesc în primul răspuns HTML, inclusiv când JavaScript este dezactivat. Calculatorul preia aceeași comparație la pornire, apoi o actualizează o dată pe minut.

Serverul reutilizează comparațiile timp de 60 de secunde și le reînnoiește periodic. Colectarea surselor rămâne la 15 minute în API. Un eșec al API-ului păstrează ultimele valori reale și orele lor, cu avertizare de date expirate; fără o valoare validă apare o stare explicită de indisponibilitate. Nu se generează cursuri de exemplu în producție. Ofertele preferențiale rămân separate de tabelul standard, iar calculatorul verifică suma și categoria din linkurile paginilor.

Paginile valutare compară 5.000 RON → valută și 1.000 unități de valută → RON. Categoria bănci, brokeri sau numerar restrânge furnizorii afișați, fără să refacă formulele financiare în frontend. Pagina București acoperă numai Tavex și Luxor București și trimite la sursele oficiale pentru locații și program.

| Pagină | Intenția principală de căutare |
| --- | --- |
| `/` | comparator curs valutar, calculator pentru suma proprie |
| `/cel-mai-bun-curs-valutar/` | unde schimb bani la cel mai bun curs valutar |
| `/curs-eur-ron/` | curs euro EUR/RON, cumpărare și vânzare |
| `/curs-usd-ron/` | curs dolar USD/RON |
| `/curs-gbp-ron/` | curs liră sterlină GBP/RON |
| `/curs-chf-ron/` | curs franc elvețian CHF/RON |
| `/curs-valutar-banci/` | comparație curs valutar bănci |
| `/curs-valutar-brokeri/` | curs valutar brokeri și fintech |
| `/case-schimb-valutar-bucuresti/` | schimb euro București, cursuri și locații |

Acestea sunt intenții alese pentru conținutul disponibil, nu estimări ale volumului de căutare. Prioritizarea ulterioară se face din Search Console: România, interogări și pagini, impresii, clicuri și poziție, comparând perioade de cel puțin 28 de zile.

### Pornire frontend și Render

Din `frontend`, rulează `npm ci`, `npm run build`, apoi `npm start`. Serverul folosește `PORT` (implicit 5173) și `API_URL` (implicit `http://localhost:8080/api/v1`). Rutele `/api/v1/comparison` și `/api/v1/history` sunt proxiate către API, astfel încât browserul poate folosi aceeași origine. `VITE_API_URL` rămâne compatibil ca alternativă pentru instalările existente. În dezvoltare, Vite proxiază `/api` spre `localhost:8080`.

Pentru Render cu Node, folosește un **Web Service**, directorul rădăcină `frontend`, comanda de build `npm ci && npm run build`, comanda de start `npm start` și variabila `API_URL` setată la URL-ul API-ului cu sufixul `/api/v1`. Ruta de health check este `/healthz`. Imaginea Docker a frontendului include deja serverul; Docker Compose îi setează adresa internă a API-ului.

Un **Render Static Site** care publică doar `dist` nu poate reînnoi HTML-ul la cerere. Buildul poate include o comparație dacă API-ul este accesibil atunci, dar pentru rate actualizate în primul HTML trebuie folosit Web Service-ul de mai sus. Acest schimb de cod nu modifică automat serviciile din Render. Păstrează adresa publică actuală la deploy; dacă adresa se schimbă, actualizează canonical-urile, sitemapul și redirecționează permanent vechile URL-uri înainte de publicare.

Validare: `npm run build`, `npm run test:seo` și `npm run test:chart`. Testele SEO pornesc un API local controlat și verifică răspunsurile HTML, filtrarea, cache-ul, eșecurile și siguranța datelor incluse în pagină.
