# MVP Demo Plan

Consumer Market Research Example
example seller datasets
- github.com/luminati-io/Amazon-dataset-samples

- luminati-io/Walmart-dataset-samples

- luminati-io/Google-Shopping-dataset-sample

- https://insideairbnb.com/get-the-data/

- http://zillow.com/research/data/

- redfin market data
- http://zillow.com/research/data/
- luminati-io/Airbnb-dataset-samples

run buyer agent queries like:
- “Which retailer has the most aggressive pricing on consumer electronics under $100?”
- “Which zip codes in Austin have the highest spread between long-term rental yield and short-term rental revenue potential, where inventory is rising and days on market are above 30?”


An analyst asks: “What’s the median gross margin for Series B SaaS companies selling to healthcare, with $5-15M ARR?” Nobody publishes this. But hundreds of CFOs, controllers, and fractional finance people at these companies know the answer for their company. An exchange that aggregates anonymized submissions from 15-20 of them creates something genuinely new. You could fake this with a synthetic dataset of 50 private SaaS companies with revenue, margins, headcount, burn rate, and vertical. The demo question becomes something a PE or growth equity analyst would ask every week and currently answers by paying $50K/year for PitchBook plus hours of manual triangulation.

What’s the actual foot traffic like at the new mixed-use development on 12th Street in Nashville — is the retail ground floor leasing up or sitting empty?

also run one example of a failed to find, buyers agent posts request for information, seller fills it later.

basic agent setup:
- tool calling llm with access to sellers data api
- sellers data in sql table and basic data reports
- sellers trust score and cost 
- website has to: chat interface. doesnt need seller interface yet.