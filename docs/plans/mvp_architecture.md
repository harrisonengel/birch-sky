# Overview

## Cloud
- Build with AWS. 
- Start with minimal fanciness, but enough to scale to 1000 seller and 10 buyers.

## demo
make demo shareable, so this all needs to be built securely on the cloud from the start

## Market
- Source of truth where seller data is posted (start with cheap, simple doc storage and an sql table)
- Vector and search engine (opensearch, or maybe one of the newer vector searches)
- Stripe for payments
- can input seller data manually for demo

## Architecture
[Web Front End] --> [Buyers Agent Platform] --> [Market Platform]

### Front End
- for the demo, only buyer front end
- TODO: get an 8 bit pixel art of the buyers agent
- text input should be a simple llm chat box to start, with the 8bit buyers agent standing and waiting
- when you send the request he marches off to an 8-bit data market thst looks like a stock market building
- you get a little feedback bar that he is working on your behalf
- then he comes back, with a list of info to buy
- you chose the sources and have a payment navigation. TODO: payment should feel a lot like biying on a stock app like robinhood

### Buyers Agent Platform
- Buyers agent is spun off for a request
- this is a literal llm agent, like claude. it takes the users exact query as its only request. we will put it in more context including mcps it has access to.
- the agent runs in a sandbox with only access to the market platform search and analyze tools
- the output of the run is a summary of the success of the agent and a list of sources for sale it used
- or, if it was unsuccessful, suggestion to post a buy order for information


