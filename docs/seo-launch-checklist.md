# Lansare și creștere SEO pentru UndeSchimb

Schimbările din repository acoperă indexarea tehnică, conținutul crawlabil, datele structurate, legăturile interne, transparența și accesul crawlerelor de căutare. Poziția în Google nu poate fi garantată și mai are nevoie de semnale externe, date reale de utilizare și timp.

## La fiecare lansare

1. Rulează `npm run build` în `frontend`. Build-ul verifică automat titlurile, descrierile, canonicalele, JSON-LD-ul, sitemap-ul și legăturile dintre paginile statice.
2. Verifică în producție `/robots.txt` și `/sitemap.xml`. Toate URL-urile din sitemap trebuie să răspundă cu `200`, fără redirecționare către homepage.
3. Testează homepage-ul și cel puțin o pagină statică în [PageSpeed Insights](https://pagespeed.web.dev/), pe mobil. Urmărește LCP sub 2,5 s, INP sub 200 ms și CLS sub 0,1 la percentila 75.
4. După modificarea reală a unei pagini, actualizează `dateModified` din JSON-LD și `<lastmod>` din sitemap. Nu schimba datele doar pentru a simula prospețime.

## Acțiuni care necesită acces la conturile proprietarului

1. Adaugă domeniul în [Google Search Console](https://search.google.com/search-console/about), verifică proprietatea și trimite `https://undeschimb.onrender.com/sitemap.xml`.
2. Inspectează URL-urile principale și solicită indexarea o singură dată după lansare: homepage, cele patru pagini valutare, ghidul BNR și ghidurile pentru bănci, brokeri și case de schimb.
3. Configurează [Bing Webmaster Tools](https://www.bing.com/webmasters/about) și trimite același sitemap. Activează IndexNow numai după generarea și găzduirea cheii asociate site-ului.
4. Adaugă în paginile „Despre” sau „Transparență” un autor ori responsabil editorial și o metodă reală de contact, dacă dorești să le publici. Nu folosi identități sau adrese inventate.
5. Dacă treci la un domeniu propriu, modifică în aceeași lansare canonicalele, URL-urile Open Graph, sitemap-ul și linia `Sitemap` din `robots.txt`; apoi adaugă redirecționări 301 de la domeniul vechi.

## Google și căutare asistată de AI

- Nu bloca `Googlebot`, `Bingbot`, `OAI-SearchBot` sau `PerplexityBot`. Regulile explicite se află în `frontend/public/robots.txt`.
- Pentru vizibilitate în ChatGPT Search este relevant `OAI-SearchBot`. `GPTBot` controlează separat folosirea pentru antrenare; alegerea de a-l permite sau bloca este o decizie de politică, nu o tehnică de clasare.
- Nu este necesar un fișier `llms.txt` pentru Google sau ChatGPT Search. Menține în schimb HTML semantic, surse verificabile, canonicale corecte și răspunsuri clare în paginile vizibile.
- Urmărește în Search Console interogările și paginile care primesc afișări. Extinde doar subiectele pentru care poți adăuga informații originale, nu pagini aproape identice generate pentru fiecare variație de cuvânt-cheie.

## Creștere editorială recomandată

1. Publică lunar un raport original bazat pe istoricul din baza de date: diferența medie față de BNR, amplitudinea spreadului și numărul de surse disponibile. Explică metoda și evită concluziile de investiții.
2. Creează pagini dedicate ofertelor speciale numai după verificarea sursei oficiale, a programului, limitei și datei ultimei revizuiri. Leagă-le de oferta din comparator.
3. Obține mențiuni editoriale relevante de la publicații românești de finanțe personale, comunități de expați, călătorii și investiții. Nu cumpăra pachete de linkuri și nu publica advertoriale mascate.
4. Revizuiește trimestrial paginile de metodologie, transparență și confidențialitate, precum și toate sursele furnizorilor.

## Surse oficiale de urmărit

- [Google: AI features and your website](https://developers.google.com/search/docs/fundamentals/ai-optimization-guide)
- [Google: helpful, reliable, people-first content](https://developers.google.com/search/docs/fundamentals/creating-helpful-content)
- [Google: Core Web Vitals](https://developers.google.com/search/docs/appearance/core-web-vitals)
- [OpenAI Docs: crawler controls](https://developers.openai.com/api/docs/bots)
- [Perplexity: crawler documentation](https://docs.perplexity.ai/docs/resources/perplexity-crawlers)
- [Bing: IndexNow](https://www.bing.com/webmasters/help/indexnow-0z209wby)
