import unittest
from worker import Extractor, clean, valid_id

VIDEO='abcdefghijk'
class Fake:
    def get_song(self, video_id):
        return {'videoDetails': {'videoId':video_id,'title':'Song','viewCount':'12'}, 'streamingData':{'url':'private'}}
    def get_watch_playlist(self, **kwargs):
        return {'tracks':[{'videoId':VIDEO,'album':{'id':'MPREexample'},'feedbackTokens':{'private':'secret'}},{'videoId':'wrongrecord','title':'Another song'}],'lyrics':'PRIVATE_POINTER'}
    def get_album(self, entity_id):
        return {'title':'Album','trackCount':5,'tracks':[{'title':str(i)} for i in range(5)],'new_extra':'keep'}
    def get_artist(self, entity_id):
        return {'name':'Artist','description':'Bio','monthlyListeners':'123','songs':{'results':[{'title':'Song'}]*5}}

class Tests(unittest.TestCase):
    def test_song_keeps_only_exact_recording_and_no_private_data(self):
        result=Extractor(Fake()).execute('song',VIDEO)
        self.assertEqual(len(result['watch']['tracks']),1)
        self.assertNotIn('feedbackTokens',str(result))
        self.assertNotIn('lyrics',result['watch'])
        self.assertEqual(result['song']['videoDetails']['viewCount'],'12')
    def test_player_failure_retains_watch_metadata(self):
        class Unavailable(Fake):
            def get_song(self, video_id): raise RuntimeError('secret')
        result=Extractor(Unavailable()).execute('song',VIDEO)
        self.assertTrue(result['_partial'])
        self.assertEqual(result['watch']['tracks'][0]['videoId'],VIDEO)
    def test_album_and_artist_limits_preserve_extra_metadata(self):
        album=Extractor(Fake()).execute('album','MPREexample',2)
        self.assertEqual(len(album['metadata']['tracks']),2)
        self.assertEqual(album['metadata']['new_extra'],'keep')
        self.assertTrue(album['truncated'])
        artist=Extractor(Fake()).execute('artist','UC'+'x'*22,2)
        self.assertEqual(len(artist['metadata']['songs']['results']),2)
        self.assertEqual(artist['metadata']['monthlyListeners'],'123')
    def test_bounded_artist_release_traversal(self):
        class Releases(Fake):
            def get_artist(self, entity_id):
                return {'albums': {'browseId':'BROWSE', 'params':'PARAMS', 'results':[]}}
            def get_artist_albums(self, browse_id, params, limit):
                self.arguments=(browse_id, params, limit)
                return [{'title':str(i)} for i in range(5)]
        client=Releases()
        result=Extractor(client).execute('artist_albums','UC'+'x'*22,2)
        self.assertEqual(client.arguments,('BROWSE','PARAMS',2))
        self.assertEqual(len(result['results']),2)
        self.assertTrue(result['possibly_more'])
    def test_private_fields_and_identifier_rejection(self):
        self.assertEqual(clean({'authorization':'secret','feedbackTokens':'secret','safe':3}),{'safe':3})
        self.assertFalse(valid_id('song','https://evil.test'))
        with self.assertRaises(ValueError):Extractor(Fake()).execute('song','invalid')

if __name__=='__main__':unittest.main()
