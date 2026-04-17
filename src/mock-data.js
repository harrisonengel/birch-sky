export function generateBuyRequest(query) {
  return {
    title: query.length > 60 ? query.slice(0, 60) + '...' : query,
    description: `Seeking data to answer: "${query}". Require current data with verifiable sourcing.`,
    maxPrice: '5.00',
    expiration: '7 days',
  };
}
