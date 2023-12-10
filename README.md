defo gotta figure out wtf about error handling -- like... what should we do? easier for p2p things
but basically if we have something go wrong i think we should try to restart hte segment worker...
if we ultimately cant get the segment worker running we shoudl put the interface to down (like link set down or w/e)

should have links start in down position and then moved to up if/when segment worker is ... working