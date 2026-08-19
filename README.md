# UndeSchimb

UndeSchimb compară cursurile standard pentru schimburi între conturi personale în România, raportând rezultatul la cursul de referință BNR.

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

Ratele BNR sunt un reper informativ, nu o ofertă executabilă. XTB este marcat ca estimativ: aplicația folosește cotații publice Standard și comisionul de conversie publicat de XTB.
