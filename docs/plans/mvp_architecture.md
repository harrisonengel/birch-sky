
# Top Level Technology and Provider Decisions
- Coding language for the backend services is Go lang
- Build this on AWS, all demoable steps should be serveable and a real deployed product when I start showing it off.
- Stripe for payments

## Cloud Setup

- Demo MVP should work well enough to scale to O(1000) sellers (so a few million docs and hundreds of SQL tables) and a few hundred buyers.

## demo
make demo shareable, so this all needs to be built securely on the cloud from the start

## Market
- Source of truth where seller data is posted (start with cheap, simple doc storage and an sql table)
- Vector and search engine (opensearch, or maybe one of the newer vector searches)
- Stripe for payments
- can input seller data manually for demo

# Architecture
[Web Front End] --> [Buyers Agent Platform] --> [Market Platform]

## Buyer Front End
- Text box, above an 8 bit pixel art of a "buyers agent", a man in a suite and hat like a 1920s sockbroker. He's standing below the right side of the text box waiting for your instructions.
- To the right of him, after some distance he will have to run, you see an 8-bit bixel art building that looks like the new york stock exchange and has the title Information Exchange above it.
- text input should be a simple llm chat box to start, with the 8bit buyers agent standing and waiting. He has two movement positions waiting for you, so he appears to bounce like old videogame pixel art.
- when you send the request he runs off to the 8-bit data market.
- The animation of the Information Exchange bounces more quickly to show he is working on it.
- then the buyers agent comes back, with a list of info to buy
- you chose the sources and have a payment navigation. TODO: payment should feel a lot like biying on a stock app like robinhood
- if there is no good info, he comes back and suggests you post a buy request. he gives you a starting request based on your initial query and what data he couldnt find. (see more details about buy requests below). seller has price they post, a lot like a bid in a stock market app.
- all of this of course is UI glamor over backend service calls

## IE Agent Platform
The back end platform.
Json request based APIs to access

### Buyers Agent Buy Flow
- Buyers agent is spun off for a request
- this is a literal llm agent, like claude. it takes the users exact query as its only request. we will put it in more context including mcps it has access to.
- the agent runs in a sandbox with only access to the market platform search and analyze tools
- the output of the run is a summary of the success of the agent and a list of sources for sale it used
- or, if it was unsuccessful, suggestion to post a buy order for information

### Buy Order for information
- place buy order can be called after a failed buyers agent flow
- we take the evaluation criteria given by the caller, wrap our the default criteria (eg data must be current time wise), and package into a BuyRequesr
- BuyRequest contains the evaluation criteria and price for fullfilment
- TODO: for demo, we will have a BuyOrder get filled by cron job and http request. full buy order fullfilment design is tbd.

## Payment Flow
- since the buyer agent comes back with the public descriptions of data to purchase and costs, payment doesnt need to go through the agent platform
- instead, it goes straight through the market platform. 
- market platform takes payment via stripe, verifies payment succeeds, then returns the data to the buyer and associates the data as "owned" by the buyer (so the dont have to re-buy it)

## Market Platform
The market platform encapsulates all of the tools needed to sell, buy, find, and analyze data on the market. It will ONLY be accessible via agents on the buy side or internal calls on the sell side (sellers only see their own data).
TODO: Decide which storage solution is good for a working, sellable, demo-able MVP.
- Data for sale will exist as raw file storage or sql table to start.
- Data for sale will be uploaded manually by me for the first few customers and for the demo. 
- Data for sale will be indexed in OpenSearch. The contents of files are indexed, and so are descriptions of SQL tables.
Draft schema for data for sale:
'''
SalesInfoUnit {
   ID string
   Cost (TODO: this is complex as some data may be pay-for-file and others are pay-for-access. For now it should be one price and per-access is 24 hours)
   Seller ID (for payment)
   Expiration Date (optional)
   Entry timestamp
   updated timestamp
   SQL Details{
       Table ID
       summary of data for sale
   }
   Document Details{
       Doc ID
       Doc Literal file bytes
       Doc Parsed Content
   }
}
'''
- This is the source-of-truth for the definition of some data for sale in the system


### Search platform
- Search platform will expose hybrid text and embeddings based search.
- To start, the buyers agent will have direct access to query opensearch. Part of the additional context the buyers agent is given is the OpenSearch document schema it can query.
- 

# Environment Separation
We'll have two completely separate environments from the start for the entire applicaton. One Test, and one Prod. The demo will be shown on the Test environment.